package model

type SearchResult struct {
	RunID    int64
	JobID    int64
	JobName  string
	StepName string
	Line     int
	Content  string
}

type SearchQuery struct {
	Pattern       string
	IsRegex       bool
	CaseSensitive bool
	FailedOnly    bool
	StepFilter    string
	JobPattern    string
	ContextLines  int
}

type SearchResults struct {
	Query      SearchQuery
	Matches    []SearchResult
	JobCounts  map[string]int // job name -> match count
	TotalCount int
}
