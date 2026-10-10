package analyzer

import (
	"fmt"
	"regexp"
	"strings"
)

var (
	ktForceUnwrap     = regexp.MustCompile(`\b([a-zA-Z_]\w*)!!`)
	ktNullVarDecl     = regexp.MustCompile(`\b(?:val|var)\s+([a-zA-Z_]\w*)\s*(?::\s*[^=]+)?\s*=\s*null\b`)
)

// AnalyzeKotlin provides dedicated static analysis for Kotlin code.
func AnalyzeKotlin(lines []string) []Issue {
	var issues []Issue

	issues = append(issues, CheckLanguageBracketBalance(lines, "kotlin")...)
	issues = append(issues, CheckCommonLogicalBugs(lines, "kotlin")...)

	nullVars := make(map[string]int)

	for lineIdx, line := range lines {
		lineNum := lineIdx + 1
		trimmed := strings.TrimSpace(line)

		if trimmed == "" || strings.HasPrefix(trimmed, "//") || strings.HasPrefix(trimmed, "/*") {
			continue
		}

		// Track nullable variables assigned to null
		if m := ktNullVarDecl.FindStringSubmatch(line); len(m) > 1 {
			nullVars[m[1]] = lineNum
		}

		// Detect !! on known null variable
		for varName, declLine := range nullVars {
			if strings.Contains(line, varName+"!!") && lineNum > declLine {
				issues = append(issues, Issue{
					Line:       lineNum,
					Severity:   "error",
					Category:   "NullPointerException Hazard",
					Rule:       "KOTLIN_NULL_POINTER_EXCEPTION",
					Message:    fmt.Sprintf("Force unwrapping with '!!' on '%s' (set to null on Line %d) causes KotlinNullPointerException.", varName, declLine),
					Snippet:    line,
					Suggestion: fmt.Sprintf("Use safe call operator '%s?.property' or Elvis operator '?:' instead of '!!'.", varName),
				})
			}
		}

		// General warning on !! operator
		if ktForceUnwrap.MatchString(line) {
			matches := ktForceUnwrap.FindAllStringSubmatch(line, -1)
			for _, m := range matches {
				if len(m) > 1 && nullVars[m[1]] == 0 {
					issues = append(issues, Issue{
						Line:       lineNum,
						Severity:   "warning",
						Category:   "Null Safety Hazard",
						Rule:       "KOTLIN_FORCE_UNWRAP",
						Message:    fmt.Sprintf("Using force unwrap operator '!!' on '%s' bypasses Kotlin null safety and may crash at runtime.", m[1]),
						Snippet:    line,
						Suggestion: fmt.Sprintf("Replace with safe call '%s?.' or Elvis operator '?:'.", m[1]),
					})
				}
			}
		}
	}

	return deduplicateIssues(issues)
}
