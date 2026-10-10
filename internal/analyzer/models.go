package analyzer

// AnalyzeRequest holds the payload received from the frontend client.
type AnalyzeRequest struct {
	Code     string `json:"code"`
	Language string `json:"language"`
}

// Issue represents an error, bug, or warning identified at a specific line.
type Issue struct {
	Line       int    `json:"line"`
	Column     int    `json:"column,omitempty"`
	EndLine    int    `json:"end_line,omitempty"`
	Severity   string `json:"severity"`    // "error", "warning", "info"
	Category   string `json:"category"`    // "Syntax Error", "Logic Bug", "Runtime Crash", etc.
	Rule       string `json:"rule"`        // e.g., "SYNTAX_UNCLOSED_BRACKET"
	Message    string `json:"message"`     // Human friendly explanation
	Snippet    string `json:"snippet"`     // Source code extract
	Suggestion string `json:"suggestion"`  // Suggested correction
}

// Summary provides aggregate metrics for the analysis run.
type Summary struct {
	TotalErrors     int     `json:"total_errors"`
	TotalWarnings   int     `json:"total_warnings"`
	TotalInfos      int     `json:"total_infos"`
	ExecutionTimeMs float64 `json:"execution_time_ms"`
	Status          string  `json:"status"` // "PASSED", "WARNINGS_FOUND", "ERRORS_FOUND"
}

// AnalyzeResponse is returned to the frontend.
type AnalyzeResponse struct {
	Success    bool    `json:"success"`
	Language   string  `json:"language"`
	TotalLines int     `json:"total_lines"`
	Issues     []Issue `json:"issues"`
	Summary    Summary `json:"summary"`
}
