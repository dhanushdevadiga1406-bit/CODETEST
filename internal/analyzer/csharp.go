package analyzer

import (
	"fmt"
	"regexp"
	"strings"
)

var (
	csNullDecl         = regexp.MustCompile(`\b(?:string|int\?|double\?|var|object|[A-Z][A-Za-z0-9_]*)\s+([A-Za-z_]\w*)\s*=\s*null\s*;`)
	csMissingSemi      = regexp.MustCompile(`^\s*(Console\.WriteLine|return|int|string|var|bool|double)\b`)
	csLinqFirstHazard  = regexp.MustCompile(`\.First\(\)`)
	csNullableUnwrap   = regexp.MustCompile(`\bint\s+([a-zA-Z0-9_]+)\s*=\s*([a-zA-Z0-9_]+)\.Value\s*;`)
)

// AnalyzeCSharp provides dedicated static analysis for C# (.NET) programs.
func AnalyzeCSharp(lines []string) []Issue {
	var issues []Issue

	issues = append(issues, CheckLanguageBracketBalance(lines, "csharp")...)
	issues = append(issues, CheckCommonLogicalBugs(lines, "csharp")...)

	nullVars := make(map[string]int)

	for lineIdx, line := range lines {
		lineNum := lineIdx + 1
		trimmed := strings.TrimSpace(line)

		if trimmed == "" || strings.HasPrefix(trimmed, "//") || strings.HasPrefix(trimmed, "/*") || strings.HasPrefix(trimmed, "*") {
			continue
		}

		// --- C# NullReferenceException Detection ---
		if m := csNullDecl.FindStringSubmatch(line); len(m) > 1 {
			nullVars[m[1]] = lineNum
		}
		for varName, declLine := range nullVars {
			// Member access on null variable: varName.Property or varName.Method()
			derefPattern := regexp.MustCompile(`\b` + regexp.QuoteMeta(varName) + `\.[a-zA-Z0-9_]+`)
			if derefPattern.MatchString(line) && lineNum > declLine && !strings.Contains(line, varName+" = new") && !strings.Contains(line, varName+"?.") {
				issues = append(issues, Issue{
					Line:       lineNum,
					Severity:   "error",
					Category:   "NullReferenceException Hazard",
					Rule:       "CS_NULL_REFERENCE_EXCEPTION",
					Message:    fmt.Sprintf("Dereferencing '%s' will throw a NullReferenceException at runtime; variable was assigned null on Line %d.", varName, declLine),
					Snippet:    line,
					Suggestion: fmt.Sprintf("Use the null-conditional operator ('%s?.Property') or verify 'if (%s != null)'.", varName, varName),
				})
			}
		}

		// LINQ .First() on potentially empty sequence throws InvalidOperationException
		if csLinqFirstHazard.MatchString(line) {
			issues = append(issues, Issue{
				Line:       lineNum,
				Severity:   "warning",
				Category:   "Runtime Crash Hazard",
				Rule:       "CS_LINQ_FIRST_EMPTY_SEQUENCE",
				Message:    "Using .First() throws InvalidOperationException if sequence is empty.",
				Snippet:    line,
				Suggestion: "Use .FirstOrDefault() and handle null / default return value.",
			})
		}

		// Missing semicolon in statement
		if csMissingSemi.MatchString(trimmed) &&
			!strings.HasSuffix(trimmed, ";") && !strings.HasSuffix(trimmed, "{") && !strings.HasSuffix(trimmed, "}") &&
			!strings.HasSuffix(trimmed, ":") && !strings.HasSuffix(trimmed, "\\") {
			if strings.Contains(line, "=") || strings.Contains(line, "Console.WriteLine") || strings.Contains(line, "return ") {
				issues = append(issues, Issue{
					Line:       lineNum,
					Severity:   "error",
					Category:   "Syntax Error",
					Rule:       "CS_MISSING_SEMICOLON",
					Message:    "Missing terminating semicolon ';' at the end of C# statement.",
					Snippet:    line,
					Suggestion: "Add ';' to the end of the line.",
				})
			}
		}
	}

	return deduplicateIssues(issues)
}
