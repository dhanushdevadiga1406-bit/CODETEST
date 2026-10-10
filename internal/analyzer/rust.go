package analyzer

import (
	"fmt"
	"regexp"
	"strings"
)

var (
	rustUnwrapHazard   = regexp.MustCompile(`\b([a-zA-Z_]\w*)\.unwrap\(\)`)
	rustNoneOptionDecl = regexp.MustCompile(`\blet\s+(?:mut\s+)?([a-zA-Z_]\w*)\s*(?::\s*Option<[^>]+>)?\s*=\s*None\s*;`)
	rustMissingFnSemi  = regexp.MustCompile(`^\s*(println!|let\s+)\b`)
)

// AnalyzeRust provides dedicated static analysis for the Rust programming language.
func AnalyzeRust(lines []string) []Issue {
	var issues []Issue

	issues = append(issues, CheckLanguageBracketBalance(lines, "rust")...)
	issues = append(issues, CheckCommonLogicalBugs(lines, "rust")...)

	noneVars := make(map[string]int)

	for lineIdx, line := range lines {
		lineNum := lineIdx + 1
		trimmed := strings.TrimSpace(line)

		if trimmed == "" || strings.HasPrefix(trimmed, "//") || strings.HasPrefix(trimmed, "/*") || strings.HasPrefix(trimmed, "*") {
			continue
		}

		// Track Option variables initialized to None
		if m := rustNoneOptionDecl.FindStringSubmatch(line); len(m) > 1 {
			noneVars[m[1]] = lineNum
		}

		// Check unwrap on known None variable
		for varName, declLine := range noneVars {
			if strings.Contains(line, varName+".unwrap()") && lineNum > declLine && !strings.Contains(line, varName+" = Some") {
				issues = append(issues, Issue{
					Line:       lineNum,
					Severity:   "error",
					Category:   "Option None Panic Hazard",
					Rule:       "RUST_UNWRAP_ON_NONE",
					Message:    fmt.Sprintf("Calling .unwrap() on '%s' (set to None on Line %d) causes fatal runtime panic.", varName, declLine),
					Snippet:    line,
					Suggestion: fmt.Sprintf("Use 'if let Some(val) = %s' or match expression instead of .unwrap().", varName),
				})
			}
		}

		// General unwrap() warning in production Rust code
		if rustUnwrapHazard.MatchString(line) && !strings.Contains(line, "#[test]") {
			matches := rustUnwrapHazard.FindAllStringSubmatch(line, -1)
			for _, m := range matches {
				if len(m) > 1 && noneVars[m[1]] == 0 {
					issues = append(issues, Issue{
						Line:       lineNum,
						Severity:   "warning",
						Category:   "Panic Hazard",
						Rule:       "RUST_UNCHECKED_UNWRAP",
						Message:    fmt.Sprintf("Calling .unwrap() on '%s' will panic if the Option is None or Result is Err.", m[1]),
						Snippet:    line,
						Suggestion: "Handle potential None/Err with '?', 'unwrap_or()', or 'match'.",
					})
				}
			}
		}

		// Missing semicolon in statement lines
		if rustMissingFnSemi.MatchString(trimmed) && !strings.HasSuffix(trimmed, ";") && !strings.HasSuffix(trimmed, "{") && !strings.HasSuffix(trimmed, "}") {
			issues = append(issues, Issue{
				Line:       lineNum,
				Severity:   "error",
				Category:   "Syntax Error",
				Rule:       "RUST_MISSING_SEMICOLON",
				Message:    "Missing terminating semicolon ';' at the end of statement in Rust.",
				Snippet:    line,
				Suggestion: "Add ';' to terminate the statement.",
			})
		}
	}

	return deduplicateIssues(issues)
}
