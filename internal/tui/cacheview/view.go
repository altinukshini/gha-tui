package cacheview

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/altin/gh-actions-tui/internal/model"
	"github.com/altin/gh-actions-tui/internal/ui"
)

type cacheItem struct {
	entry    model.ActionsCache
	selected bool
}

func (c cacheItem) Title() string {
	mark := " "
	if c.selected {
		mark = ui.StyleWarning.Render("● ")
	}
	size := ui.StyleWarning.Render(formatSize(c.entry.SizeInBytes))
	return fmt.Sprintf("%s%s  %s", mark, c.entry.Key, size)
}

func (c cacheItem) Description() string {
	parts := []string{}

	// Branch from ref (strip refs/heads/ or refs/pull/)
	branch := c.entry.Ref
	if strings.HasPrefix(branch, "refs/heads/") {
		branch = strings.TrimPrefix(branch, "refs/heads/")
	} else if strings.HasPrefix(branch, "refs/pull/") {
		branch = "PR " + strings.TrimPrefix(branch, "refs/pull/")
		branch = strings.TrimSuffix(branch, "/merge")
	}
	if branch != "" {
		parts = append(parts, ui.StyleInfo.Render(branch))
	}

	if !c.entry.CreatedAt.IsZero() {
		parts = append(parts, ui.StyleMuted.Render("cached "+relativeTime(c.entry.CreatedAt)))
	}
	if !c.entry.LastAccessedAt.IsZero() {
		parts = append(parts, ui.StyleMuted.Render("last used "+relativeTime(c.entry.LastAccessedAt)))
	}

	return strings.Join(parts, "  ")
}

func (c cacheItem) FilterValue() string {
	return c.entry.Key + " " + c.entry.Ref
}

// SortMode determines how cache entries are ordered.
type SortMode int

const (
	SortByAccessed SortMode = iota
	SortByDate
	SortBySize
)

func (s SortMode) String() string {
	switch s {
	case SortByAccessed:
		return "last used"
	case SortByDate:
		return "created"
	case SortBySize:
		return "size"
	default:
		return "last used"
	}
}

// Model is the cache management list view.
type Model struct {
	list       list.Model
	entries    []model.ActionsCache
	selected   map[int64]bool
	totalCount int
	sortMode   SortMode
	totalSize  int64
	width      int
	height     int
	loading    bool
	err        error
}

// New creates a cache management view.
func New() Model {
	delegate := list.NewDefaultDelegate()
	delegate.SetHeight(2)
	delegate.SetSpacing(0)

	l := list.New(nil, delegate, 0, 0)
	l.SetShowTitle(false)
	l.SetShowHelp(false)
	l.SetShowStatusBar(true)
	l.SetFilteringEnabled(true)
	l.KeyMap.Filter = key.NewBinding(key.WithKeys("f"), key.WithHelp("f", "filter"))
	l.DisableQuitKeybindings()

	return Model{list: l, selected: make(map[int64]bool), loading: true}
}

func (m Model) Init() tea.Cmd { return nil }

func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	switch msg := msg.(type) {
	case ui.ActionsCachesLoadedMsg:
		m.loading = false
		if msg.Err != nil {
			m.err = msg.Err
			return m, nil
		}
		m.entries = msg.Caches
		m.totalCount = msg.TotalCount
		m.selected = make(map[int64]bool)
		m.totalSize = 0
		for _, e := range m.entries {
			m.totalSize += e.SizeInBytes
		}
		m.sortEntries()
		cmd := m.list.SetItems(m.buildItems())
		return m, cmd

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		// Reserve one line for the header.
		m.list.SetSize(msg.Width, msg.Height-1)

	case tea.KeyMsg:
		if msg.String() == " " && !m.IsFiltering() {
			if item, ok := m.list.SelectedItem().(cacheItem); ok {
				id := item.entry.ID
				if m.selected[id] {
					delete(m.selected, id)
				} else {
					m.selected[id] = true
				}
				cmd := m.list.SetItems(m.buildItems())
				return m, cmd
			}
			return m, nil
		}
		if msg.String() == "s" && !m.IsFiltering() {
			m.sortMode = (m.sortMode + 1) % 3
			m.sortEntries()
			cmd := m.list.SetItems(m.buildItems())
			return m, cmd
		}
	}

	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)
	return m, cmd
}

func (m Model) View() string {
	if m.loading {
		return "\n  Loading caches..."
	}
	if m.err != nil {
		return fmt.Sprintf("\n  Error: %v\n\n  Press r to retry.", m.err)
	}
	if len(m.entries) == 0 {
		return "\n  No caches found.\n\n  This shows GitHub Actions caches (actions/cache).\n  Press r to refresh."
	}

	countLabel := fmt.Sprintf("%d caches", len(m.entries))
	if m.totalCount > len(m.entries) {
		countLabel = fmt.Sprintf("%d / %d caches", len(m.entries), m.totalCount)
	}

	header := fmt.Sprintf("  %s | Total: %s | Sort: %s | s: sort  d: delete  x: clear all",
		countLabel,
		formatSize(m.totalSize),
		m.sortMode.String(),
	)
	header = ui.StyleMuted.Render(header)

	return header + "\n" + m.list.View()
}

// SelectedEntry returns the currently selected cache entry, or nil.
func (m Model) SelectedEntry() *model.ActionsCache {
	if item, ok := m.list.SelectedItem().(cacheItem); ok {
		return &item.entry
	}
	return nil
}

// IsFiltering returns true when the user is actively typing a filter.
func (m Model) IsFiltering() bool {
	return m.list.FilterState() == list.Filtering
}

// HasActiveFilter returns true when a filter is applied.
func (m Model) HasActiveFilter() bool {
	return m.list.FilterState() != list.Unfiltered
}

// ShortHelp returns key bindings for the cache view.
func (m Model) ShortHelp() []key.Binding {
	return []key.Binding{
		key.NewBinding(key.WithKeys("s"), key.WithHelp("s", "sort")),
		key.NewBinding(key.WithKeys("d"), key.WithHelp("d", "delete")),
		key.NewBinding(key.WithKeys("x"), key.WithHelp("x", "clear all")),
	}
}

// sortEntries sorts m.entries in-place based on the current sort mode.
func (m *Model) sortEntries() {
	switch m.sortMode {
	case SortByAccessed:
		sort.Slice(m.entries, func(i, j int) bool {
			return m.entries[i].LastAccessedAt.After(m.entries[j].LastAccessedAt)
		})
	case SortByDate:
		sort.Slice(m.entries, func(i, j int) bool {
			return m.entries[i].CreatedAt.After(m.entries[j].CreatedAt)
		})
	case SortBySize:
		sort.Slice(m.entries, func(i, j int) bool {
			return m.entries[i].SizeInBytes > m.entries[j].SizeInBytes
		})
	}
}

// buildItems converts the current entries slice into list items.
func (m Model) buildItems() []list.Item {
	items := make([]list.Item, len(m.entries))
	for i, e := range m.entries {
		items[i] = cacheItem{entry: e, selected: m.selected[e.ID]}
	}
	return items
}

// SelectedCaches returns the IDs of all multi-selected caches.
func (m Model) SelectedCaches() []int64 {
	var ids []int64
	for id := range m.selected {
		ids = append(ids, id)
	}
	return ids
}

// SelectionCount returns the number of selected caches.
func (m Model) SelectionCount() int {
	return len(m.selected)
}

// ClearSelection clears all selected caches.
func (m *Model) ClearSelection() {
	for k := range m.selected {
		delete(m.selected, k)
	}
}

// formatSize formats a byte count into a human-readable string (KB, MB, GB).
func formatSize(bytes int64) string {
	const (
		kb = 1024
		mb = 1024 * kb
		gb = 1024 * mb
	)
	switch {
	case bytes >= gb:
		return fmt.Sprintf("%.1f GB", float64(bytes)/float64(gb))
	case bytes >= mb:
		return fmt.Sprintf("%.1f MB", float64(bytes)/float64(mb))
	case bytes >= kb:
		return fmt.Sprintf("%.1f KB", float64(bytes)/float64(kb))
	default:
		return fmt.Sprintf("%d B", bytes)
	}
}

// relativeTime returns a human-readable relative time string.
func relativeTime(t time.Time) string {
	d := time.Since(t)
	switch {
	case d < time.Minute:
		return "just now"
	case d < time.Hour:
		m := int(d.Minutes())
		if m == 1 {
			return "1 minute ago"
		}
		return fmt.Sprintf("%d minutes ago", m)
	case d < 24*time.Hour:
		h := int(d.Hours())
		if h == 1 {
			return "1 hour ago"
		}
		return fmt.Sprintf("%d hours ago", h)
	default:
		days := int(d.Hours() / 24)
		if days == 1 {
			return "1 day ago"
		}
		return fmt.Sprintf("%d days ago", days)
	}
}
