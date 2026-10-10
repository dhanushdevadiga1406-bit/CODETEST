package analyzer

import (
	"fmt"
	"regexp"
	"strings"
)

var (
	phpNullVarDecl    = regexp.MustCompile(`\$([a-zA-Z_\x80-\xff][a-zA-Z0-9_\x80-\xff]*)\s*=\s*null\s*;`)
	phpMissingSemi    = regexp.MustCompile(`^\s*(echo|return|\$[a-zA-Z_\x80-\xff][a-zA-Z0-9_\x80-\xff]*\s*=)\b`)
	phpMethodOnNull   = regexp.MustCompile(`\$([a-zA-Z_\x80-\xff][a-zA-Z0-9_\x80-\xff]*)\s*->\s*[a-zA-Z0-9_]+`)
)

// AnalyzePHP provides dedicated static analysis for PHP code.
func AnalyzePHP(lines []string) []Issue {
	var issues []Issue

	issues = append(issues, CheckLanguageBracketBalance(lines, "php")...)
	issues = append(issues, CheckCommonLogicalBugs(lines, "php")...)

	nullVars := make(map[string]int)

	for lineIdx, line := range lines {
		lineNum := lineIdx + 1
		trimmed := strings.TrimSpace(line)

		if trimmed == "" || strings.HasPrefix(trimmed, "//") || strings.HasPrefix(trimmed, "#") || strings.HasPrefix(trimmed, "/*") {
			continue
		}

		// Track null variables: $var = null;
		if m := phpNullVarDecl.FindStringSubmatch(line); len(m) > 1 {
			nullVars[m[1]] = lineNum
		}

		// Detect calling method or property on null variable: $var->method()
		for varName, declLine := range nullVars {
			callPattern := regexp.MustCompile(`\$` + regexp.QuoteMeta(varName) + `\s*->`)
			if callPattern.MatchString(line) && lineNum > declLine && !strings.Contains(line, "$"+varName+" = new") {
				issues = append(issues, Issue{
					Line:       lineNum,
					Severity:   "error",
					Category:   "Fatal Error on Null Object",
					Rule:       "PHP_CALL_METHOD_ON_NULL",
					Message:    fmt.Sprintf("Fatal error: Call to a member function on null ($%s was assigned null on Line %d).", varName, declLine),
					Snippet:    line,
					Suggestion: fmt.Sprintf("Use nullsafe operator '$%s?->method()' or check 'if ($%s !== null)'.", varName, varName),
				})
			}
		}

		// Missing semicolon in statement
		if phpMissingSemi.MatchString(trimmed) && !strings.HasSuffix(trimmed, ";") && !strings.HasSuffix(trimmed, "{") && !strings.HasSuffix(trimmed, "}") {
			issues = append(issues, Issue{
				Line:       lineNum,
				Severity:   "error",
				Category:   "Syntax Error",
				Rule:       "PHP_MISSING_SEMICOLON",
				Message:    "Missing terminating semicolon ';' at the end of PHP statement.",
				Snippet:    line,
				Suggestion: "Add ';' to the end of the statement.",
			})
		}
	}

	return deduplicateIssues(issues)
}
