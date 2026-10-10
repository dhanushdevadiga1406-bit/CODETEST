package analyzer

import (
	"fmt"
	"regexp"
	"strings"
)

var (
	rbBlockOpen    = regexp.MustCompile(`^\s*(?:def|class|module|if|unless|while|until|for|case)\b|\bdo\b`)
	rbBlockClose   = regexp.MustCompile(`^\s*end\b|\bend\s*$`)
	rbNullVarDecl  = regexp.MustCompile(`\b([a-zA-Z_]\w*)\s*=\s*nil\b`)
	rbEqNilCheck   = regexp.MustCompile(`\b([a-zA-Z_]\w*)\s*==\s*nil\b`)
)

// AnalyzeRuby provides dedicated static analysis for the Ruby programming language.
func AnalyzeRuby(lines []string) []Issue {
	var issues []Issue

	issues = append(issues, CheckLanguageBracketBalance(lines, "ruby")...)
	issues = append(issues, CheckCommonLogicalBugs(lines, "ruby")...)

	blockDepth := 0
	nilVars := make(map[string]int)

	for lineIdx, line := range lines {
		lineNum := lineIdx + 1
		trimmed := strings.TrimSpace(line)

		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}

		if rbBlockOpen.MatchString(trimmed) && !strings.Contains(trimmed, "end") {
			blockDepth++
		}
		if rbBlockClose.MatchString(trimmed) {
			blockDepth--
		}

		// Track nil variables
		if m := rbNullVarDecl.FindStringSubmatch(line); len(m) > 1 {
			nilVars[m[1]] = lineNum
		}

		// Calling method on nil
		for varName, declLine := range nilVars {
			callPattern := regexp.MustCompile(`\b` + regexp.QuoteMeta(varName) + `\.[a-zA-Z_]\w*`)
			if callPattern.MatchString(line) && lineNum > declLine && !strings.Contains(line, varName+".nil?") {
				issues = append(issues, Issue{
					Line:       lineNum,
					Severity:   "error",
					Category:   "NoMethodError on NilClass",
					Rule:       "RUBY_NOMETHOD_ON_NIL",
					Message:    fmt.Sprintf("Calling method on '%s' causes NoMethodError: undefined method for nil:NilClass (set to nil on Line %d).", varName, declLine),
					Snippet:    line,
					Suggestion: fmt.Sprintf("Use safe navigation operator '%s&.method' or check '%s.nil?'.", varName, varName),
				})
			}
		}

		// == nil check
		if rbEqNilCheck.MatchString(line) {
			issues = append(issues, Issue{
				Line:       lineNum,
				Severity:   "warning",
				Category:   "Style / Idiom",
				Rule:       "RUBY_EQ_NIL_STYLE",
				Message:    "Comparing to nil with '==' is not idiomatic Ruby.",
				Snippet:    line,
				Suggestion: "Use the '.nil?' predicate method instead.",
			})
		}
	}

	if blockDepth > 0 {
		issues = append(issues, Issue{
			Line:       len(lines),
			Severity:   "error",
			Category:   "Syntax Error",
			Rule:       "RUBY_MISSING_END",
			Message:    fmt.Sprintf("Missing 'end' keyword: %d block(s) were opened but not closed.", blockDepth),
			Snippet:    lines[len(lines)-1],
			Suggestion: "Add 'end' to close all opened def, class, if, or do blocks.",
		})
	}

	return deduplicateIssues(issues)
}
