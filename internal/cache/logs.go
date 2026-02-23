package cache

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
)

type LogCache struct {
	dir     string
	maxSize int64         // max total cache size in bytes
	ttl     time.Duration // cache entry TTL
}

// CacheMeta stores metadata about a cached run log entry.
type CacheMeta struct {
	RunID        int64     `json:"run_id"`
	Attempt      int       `json:"attempt"`
	WorkflowName string    `json:"workflow_name"`
	DisplayTitle string    `json:"display_title"`
	Branch       string    `json:"branch"`
	Actor        string    `json:"actor"`
	Event        string    `json:"event"`
	CreatedAt    time.Time `json:"created_at"`
	StoredAt     time.Time `json:"stored_at"`
}

// CacheEntry represents a single cached log entry with computed fields.
type CacheEntry struct {
	CacheMeta
	LastAccessed time.Time
	Size         int64
	Path         string
}

func NewLogCache(dir string, maxSizeMB int, ttl time.Duration) (*LogCache, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("create log cache dir: %w", err)
	}
	return &LogCache{
		dir:     dir,
		maxSize: int64(maxSizeMB) * 1024 * 1024,
		ttl:     ttl,
	}, nil
}

func (lc *LogCache) runDir(runID int64, attempt int) string {
	return filepath.Join(lc.dir, fmt.Sprintf("run-%d-attempt-%d", runID, attempt))
}

func (lc *LogCache) HasRun(runID int64, attempt int) bool {
	dir := lc.runDir(runID, attempt)
	info, err := os.Stat(dir)
	if err != nil {
		return false
	}
	return info.IsDir() && time.Since(info.ModTime()) < lc.ttl
}

// StoreRunLogs extracts a zip archive of run logs to the cache directory.
// Returns a map of archive entry names to local file paths.
func (lc *LogCache) StoreRunLogs(runID int64, attempt int, zipData io.Reader) (map[string]string, error) {
	data, err := io.ReadAll(zipData)
	if err != nil {
		return nil, fmt.Errorf("read zip data: %w", err)
	}

	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return nil, fmt.Errorf("open zip: %w", err)
	}

	dir := lc.runDir(runID, attempt)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("create run log dir: %w", err)
	}

	files := make(map[string]string)
	for _, f := range zr.File {
		if f.FileInfo().IsDir() {
			continue
		}
		localPath := filepath.Join(dir, filepath.Clean(f.Name))
		if err := os.MkdirAll(filepath.Dir(localPath), 0o755); err != nil {
			return nil, err
		}
		rc, err := f.Open()
		if err != nil {
			return nil, err
		}
		out, err := os.Create(localPath)
		if err != nil {
			rc.Close()
			return nil, err
		}
		_, err = io.Copy(out, rc)
		rc.Close()
		out.Close()
		if err != nil {
			return nil, err
		}
		files[f.Name] = localPath
	}
	return files, nil
}

// GetJobLog reads the full job log from the root-level file (e.g. "0_Job Name.txt"),
// falling back to concatenating step logs from the subdirectory.
func (lc *LogCache) GetJobLog(runID int64, attempt int, jobName string) (string, error) {
	dir := lc.runDir(runID, attempt)

	// Try root-level file first: {index}_{jobName}.txt
	entries, err := os.ReadDir(dir)
	if err != nil {
		return "", fmt.Errorf("read run dir: %w", err)
	}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if !strings.HasSuffix(name, ".txt") {
			continue
		}
		parsed := parseRootLogName(name)
		if parsed == jobName {
			data, err := os.ReadFile(filepath.Join(dir, name))
			if err != nil {
				return "", err
			}
			return string(data), nil
		}
	}

	// Fallback: read from subdirectory (step files)
	jobDir := filepath.Join(dir, jobName)
	stepEntries, err := os.ReadDir(jobDir)
	if err != nil {
		return "", fmt.Errorf("read job dir %s: %w", jobName, err)
	}

	var combined strings.Builder
	for _, e := range stepEntries {
		if e.IsDir() {
			continue
		}
		data, err := os.ReadFile(filepath.Join(jobDir, e.Name()))
		if err != nil {
			return "", err
		}
		combined.WriteString(fmt.Sprintf("=== %s ===\n", e.Name()))
		combined.Write(data)
		combined.WriteByte('\n')
	}
	return combined.String(), nil
}

// parseRootLogName extracts the job name from a root-level log filename.
// GitHub Actions zips contain files like "0_Build & Deploy.txt" where the
// number prefix is the job index. This returns "Build & Deploy".
func parseRootLogName(filename string) string {
	name := strings.TrimSuffix(filename, ".txt")
	// Strip leading numeric prefix + underscore: "0_Build & Deploy" -> "Build & Deploy"
	if idx := strings.Index(name, "_"); idx >= 0 {
		prefix := name[:idx]
		allDigits := true
		for _, c := range prefix {
			if c < '0' || c > '9' {
				allDigits = false
				break
			}
		}
		if allDigits {
			return name[idx+1:]
		}
	}
	return name
}

// GetAllJobLogs reads logs for all jobs in a run attempt.
// Prefers root-level files (e.g. "0_Build & Deploy.txt") which contain the
// full concatenated log for each job, falling back to subdirectory step files.
func (lc *LogCache) GetAllJobLogs(runID int64, attempt int) (map[string]string, error) {
	dir := lc.runDir(runID, attempt)
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("read run dir: %w", err)
	}

	logs := make(map[string]string)

	// First pass: read root-level job log files (full logs)
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if !strings.HasSuffix(name, ".txt") {
			continue
		}
		jobName := parseRootLogName(name)
		data, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			continue
		}
		logs[jobName] = string(data)
	}

	// If we found root-level files, use those (they have complete logs)
	if len(logs) > 0 {
		return logs, nil
	}

	// Fallback: read from subdirectories (step-level files)
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		content, err := lc.GetJobLog(runID, attempt, e.Name())
		if err != nil {
			continue
		}
		logs[e.Name()] = content
	}
	return logs, nil
}

// Evict removes expired and oversized cache entries.
func (lc *LogCache) Evict() error {
	type cacheEntry struct {
		path    string
		modTime time.Time
		size    int64
	}

	var entries []cacheEntry
	var totalSize int64

	err := filepath.Walk(lc.dir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return err
		}
		entries = append(entries, cacheEntry{path: path, modTime: info.ModTime(), size: info.Size()})
		totalSize += info.Size()
		return nil
	})
	if err != nil {
		return err
	}

	// Evict expired entries
	now := time.Now()
	remaining := entries[:0]
	for _, e := range entries {
		if now.Sub(e.modTime) > lc.ttl {
			os.Remove(e.path)
			totalSize -= e.size
		} else {
			remaining = append(remaining, e)
		}
	}
	entries = remaining

	// Evict oldest entries if over size cap
	if totalSize > lc.maxSize {
		sort.Slice(entries, func(i, j int) bool {
			return entries[i].modTime.Before(entries[j].modTime)
		})
		for _, e := range entries {
			if totalSize <= lc.maxSize {
				break
			}
			os.Remove(e.path)
			totalSize -= e.size
		}
	}
	return nil
}

// WriteMeta writes meta.json in the entry's directory.
func (lc *LogCache) WriteMeta(runID int64, attempt int, meta CacheMeta) error {
	dir := lc.runDir(runID, attempt)
	path := filepath.Join(dir, "meta.json")
	data, err := json.Marshal(meta)
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

// ReadMeta reads meta.json from a cache entry.
func (lc *LogCache) ReadMeta(runID int64, attempt int) (*CacheMeta, error) {
	dir := lc.runDir(runID, attempt)
	path := filepath.Join(dir, "meta.json")
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var meta CacheMeta
	if err := json.Unmarshal(data, &meta); err != nil {
		return nil, err
	}
	return &meta, nil
}

// ListEntries scans the cache directory and returns all entries.
func (lc *LogCache) ListEntries() ([]CacheEntry, error) {
	entries, err := os.ReadDir(lc.dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var result []CacheEntry
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		// Directory name format from runDir: run-<runID>-attempt-<attempt>
		name := e.Name()
		if !strings.HasPrefix(name, "run-") {
			continue
		}
		name = strings.TrimPrefix(name, "run-")
		idx := strings.LastIndex(name, "-attempt-")
		if idx < 0 {
			continue
		}
		runID, err1 := strconv.ParseInt(name[:idx], 10, 64)
		attempt, err2 := strconv.Atoi(name[idx+len("-attempt-"):])
		if err1 != nil || err2 != nil {
			continue
		}

		dirPath := filepath.Join(lc.dir, e.Name())
		entry := CacheEntry{
			Path: dirPath,
		}
		entry.RunID = runID
		entry.Attempt = attempt

		// Try to read metadata
		if meta, err := lc.ReadMeta(runID, attempt); err == nil {
			entry.CacheMeta = *meta
		} else {
			entry.CacheMeta.RunID = runID
			entry.CacheMeta.Attempt = attempt
		}

		// Compute size and last accessed time
		entry.Size = dirSize(dirPath)
		entry.LastAccessed = dirLastAccessed(dirPath)

		result = append(result, entry)
	}
	return result, nil
}

// DeleteEntry removes a single cache entry.
func (lc *LogCache) DeleteEntry(runID int64, attempt int) error {
	dir := lc.runDir(runID, attempt)
	return os.RemoveAll(dir)
}

// DeleteAll removes all cache entries.
func (lc *LogCache) DeleteAll() error {
	entries, err := os.ReadDir(lc.dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	for _, e := range entries {
		if e.IsDir() {
			os.RemoveAll(filepath.Join(lc.dir, e.Name()))
		}
	}
	return nil
}

// TotalSize returns total cache size in bytes.
func (lc *LogCache) TotalSize() (int64, error) {
	var total int64
	err := filepath.Walk(lc.dir, func(_ string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // skip errors
		}
		if !info.IsDir() {
			total += info.Size()
		}
		return nil
	})
	if err != nil && !os.IsNotExist(err) {
		return 0, err
	}
	return total, nil
}

func dirSize(path string) int64 {
	var size int64
	filepath.Walk(path, func(_ string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if !info.IsDir() {
			size += info.Size()
		}
		return nil
	})
	return size
}

func dirLastAccessed(path string) time.Time {
	var latest time.Time
	filepath.Walk(path, func(_ string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if info.ModTime().After(latest) {
			latest = info.ModTime()
		}
		return nil
	})
	return latest
}
