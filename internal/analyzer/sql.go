package analyzer

import (
	
	"regexp"
	"strings"
)

var (
	sqlEqNullHazard  = regexp.MustCompile(`(?i)\b(?:WHERE|AND|OR)\s+[a-zA-Z0-9_.]+\s*=\s*NULL\b`)
	sqlNeqNullHazard = regexp.MustCompile(`(?i)\b(?:WHERE|AND|OR)\s+[a-zA-Z0-9_.]+\s*!=\s*NULL\b|\b[a-zA-Z0-9_.]+\s*<>\s*NULL\b`)
	sqlUpdateNoWhere = regexp.MustCompile(`(?i)^\s*UPDATE\s+[a-zA-Z0-9_.]+\s+SET\s+[^;]+$`)
	sqlDeleteNoWhere = regexp.MustCompile(`(?i)^\s*DELETE\s+FROM\s+[a-zA-Z0-9_.]+\s*(?:;)?$`)
)

// AnalyzeSQL provides dedicated static analysis for SQL scripts.
func AnalyzeSQL(lines []string) []Issue {
	var issues []Issue

	issues = append(issues, CheckLanguageBracketBalance(lines, "sql")...)

	for lineIdx, line := range lines {
		lineNum := lineIdx + 1
		trimmed := strings.TrimSpace(line)

		if trimmed == "" || strings.HasPrefix(trimmed, "--") || strings.HasPrefix(trimmed, "/*") {
			continue
		}

		// Traps with = NULL instead of IS NULL
		if sqlEqNullHazard.MatchString(line) {
			issues = append(issues, Issue{
				Line:       lineNum,
				Severity:   "error",
				Category:   "SQL Logic Bug",
				Rule:       "SQL_EQUALS_NULL_TRAP",
				Message:    "Comparison '= NULL' evaluates to UNKNOWN / FALSE for all rows in standard SQL.",
				Snippet:    line,
				Suggestion: "Use 'IS NULL' instead of '= NULL'.",
			})
		}
		if sqlNeqNullHazard.MatchString(line) {
			issues = append(issues, Issue{
				Line:       lineNum,
				Severity:   "error",
				Category:   "SQL Logic Bug",
				Rule:       "SQL_NOT_EQUALS_NULL_TRAP",
				Message:    "Comparison '!= NULL' or '<> NULL' evaluates to UNKNOWN / FALSE in standard SQL.",
				Snippet:    line,
				Suggestion: "Use 'IS NOT NULL' instead of '!= NULL'.",
			})
		}

		// Destructive UPDATE without WHERE clause
		if sqlUpdateNoWhere.MatchString(trimmed) && !strings.Contains(strings.ToUpper(trimmed), "WHERE") {
			issues = append(issues, Issue{
				Line:       lineNum,
				Severity:   "warning",
				Category:   "Data Loss Hazard",
				Rule:       "SQL_UPDATE_WITHOUT_WHERE",
				Message:    "UPDATE statement without a WHERE clause will modify every row in the table.",
				Snippet:    line,
				Suggestion: "Add a WHERE clause to restrict the target records.",
			})
		}

		// Destructive DELETE without WHERE clause
		if sqlDeleteNoWhere.MatchString(trimmed) && !strings.Contains(strings.ToUpper(trimmed), "WHERE") {
			issues = append(issues, Issue{
				Line:       lineNum,
				Severity:   "warning",
				Category:   "Data Loss Hazard",
				Rule:       "SQL_DELETE_WITHOUT_WHERE",
				Message:    "DELETE statement without a WHERE clause will truncate/delete all rows in the table.",
				Snippet:    line,
				Suggestion: "Add a WHERE clause or use TRUNCATE TABLE if intended.",
			})
		}
	}

	return deduplicateIssues(issues)
}
