package analyzer

import (
	"fmt"
	"regexp"
	"strings"
)

var (
	swiftForceUnwrap = regexp.MustCompile(`\b([a-zA-Z_]\w*)!\.|\b([a-zA-Z_]\w*)!\s*\)`)
	swiftNilVarDecl  = regexp.MustCompile(`\b(?:var|let)\s+([a-zA-Z_]\w*)\s*(?::\s*[^=]+)?\s*=\s*nil\b`)
	swiftForceTry    = regexp.MustCompile(`\btry!\s+`)
)

// AnalyzeSwift provides dedicated static analysis for Swift code.
func AnalyzeSwift(lines []string) []Issue {
	var issues []Issue

	issues = append(issues, CheckLanguageBracketBalance(lines, "swift")...)
	issues = append(issues, CheckCommonLogicalBugs(lines, "swift")...)

	nilVars := make(map[string]int)

	for lineIdx, line := range lines {
		lineNum := lineIdx + 1
		trimmed := strings.TrimSpace(line)

		if trimmed == "" || strings.HasPrefix(trimmed, "//") || strings.HasPrefix(trimmed, "/*") {
			continue
		}

		// Track optionals assigned to nil
		if m := swiftNilVarDecl.FindStringSubmatch(line); len(m) > 1 {
			nilVars[m[1]] = lineNum
		}

		// Force unwrap on known nil optional
		for varName, declLine := range nilVars {
			if (strings.Contains(line, varName+"!.") || strings.Contains(line, varName+"!)")) && lineNum > declLine {
				issues = append(issues, Issue{
					Line:       lineNum,
					Severity:   "error",
					Category:   "Fatal Nil Unwrap Crash",
					Rule:       "SWIFT_NIL_FORCE_UNWRAP",
					Message:    fmt.Sprintf("Fatal error: Unexpectedly found nil while unwrapping Optional '%s' (set to nil on Line %d).", varName, declLine),
					Snippet:    line,
					Suggestion: fmt.Sprintf("Use optional binding 'if let val = %s' or optional chaining '%s?.property'.", varName, varName),
				})
			}
		}

		// Force try! hazard
		if swiftForceTry.MatchString(line) {
			issues = append(issues, Issue{
				Line:       lineNum,
				Severity:   "warning",
				Category:   "Runtime Crash Hazard",
				Rule:       "SWIFT_FORCE_TRY",
				Message:    "Using 'try!' crashes the application if an error is thrown.",
				Snippet:    line,
				Suggestion: "Use 'do { try ... } catch { ... }' or 'try?' to safely handle thrown errors.",
			})
		}
	}

	return deduplicateIssues(issues)
}
