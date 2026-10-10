package analyzer

import (
	"fmt"
	"regexp"
	"strings"
)

var (
	jsConstRe       = regexp.MustCompile(`\bconst\s+([a-zA-Z_$][a-zA-Z0-9_$]*)\s*=`)
	jsNanCheckRe    = regexp.MustCompile(`(?:===|==|!==|!=)\s*NaN\b|\bNaN\s*(?:===|==|!==|!=)`)
	jsTypeofMistake = regexp.MustCompile(`typeof\s+[^=!<>]+\s*===?\s*['"](str|integer|int|float|null|array|char)['"]`)
	jsDebuggerRe    = regexp.MustCompile(`\bdebugger\b`)
	jsWithRe        = regexp.MustCompile(`\bwith\s*\(`)
	jsReturnNewline = regexp.MustCompile(`^\s*return\s*$`)
	jsNullDeclaration = regexp.MustCompile(`\b(?:const|let|var)\s+([A-Za-z_$][A-Za-z0-9_$]*)\s*=\s*null\b`)
	jsNullAssignment  = regexp.MustCompile(`\b([A-Za-z_$][A-Za-z0-9_$]*)\s*=\s*null\b`)
	jsValueAssignment  = regexp.MustCompile(`\b([A-Za-z_$][A-Za-z0-9_$]*)\s*=\s*([^=;\s]+)`)
	jsPropertyAccess   = regexp.MustCompile(`\b([A-Za-z_$][A-Za-z0-9_$]*)\s*(?:\.|\[\s*["'])`)
)

// AnalyzeJavaScript inspects JavaScript / TypeScript code.
func AnalyzeJavaScript(lines []string) []Issue {
	var issues []Issue

	issues = append(issues, CheckLanguageBracketBalance(lines, "javascript")...)
	issues = append(issues, CheckCommonLogicalBugs(lines, "javascript")...)

	constVars := make(map[string]int)
	nullVars := make(map[string]int)
	isAfterReturn := false
	returnLine := 0

	for lineIdx, line := range lines {
		lineNum := lineIdx + 1
		trimmed := strings.TrimSpace(line)

		if trimmed == "" || strings.HasPrefix(trimmed, "//") || strings.HasPrefix(trimmed, "/*") {
			continue
		}

		for _, match := range jsValueAssignment.FindAllStringSubmatch(line, -1) {
			if len(match) > 2 && match[2] != "null" {
				delete(nullVars, match[1])
			}
		}
		for _, match := range jsNullDeclaration.FindAllStringSubmatch(line, -1) {
			if len(match) > 1 {
				nullVars[match[1]] = lineNum
			}
		}
		for _, match := range jsNullAssignment.FindAllStringSubmatch(line, -1) {
			if len(match) > 1 {
				nullVars[match[1]] = lineNum
			}
		}
		for _, match := range jsPropertyAccess.FindAllStringSubmatch(line, -1) {
			if len(match) < 2 {
				continue
			}
			if nullLine, knownNull := nullVars[match[1]]; knownNull {
				issues = append(issues, Issue{
					Line:       lineNum,
					Severity:   "error",
					Category:   "Runtime Crash Hazard",
					Rule:       "NULL_PROPERTY_ACCESS",
					Message:    fmt.Sprintf("Reading a property from '%s' will throw a TypeError because it is set to null on Line %d.", match[1], nullLine),
					Snippet:    line,
					Suggestion: fmt.Sprintf("Check '%s' for null before reading its property, or use optional chaining ('%s?.property').", match[1], match[1]),
				})
			}
		}

		// Detect const declarations
		matches := jsConstRe.FindAllStringSubmatch(line, -1)
		for _, m := range matches {
			if len(m) > 1 {
				constVars[m[1]] = lineNum
			}
		}

		// Detect re-assignment to const variable: varName = ... (not inside declaration)
		if !strings.Contains(line, "const ") && !strings.Contains(line, "let ") && !strings.Contains(line, "var ") {
			for varName, declLine := range constVars {
				assignPattern := regexp.MustCompile(`\b` + regexp.QuoteMeta(varName) + `\s*=(?:[^=]|$)`)
				if assignPattern.MatchString(line) && lineNum > declLine {
					issues = append(issues, Issue{
						Line:       lineNum,
						Severity:   "error",
						Category:   "Runtime Crash Hazard",
						Rule:       "CONST_REASSIGNMENT",
						Message:    fmt.Sprintf("Reassignment to constant variable '%s' declared on Line %d (TypeError).", varName, declLine),
						Snippet:    line,
						Suggestion: fmt.Sprintf("Change '%s' declaration from 'const' to 'let' if it needs reassignment.", varName),
					})
				}
			}
		}

		// Detect NaN comparison: x === NaN is always false
		if jsNanCheckRe.MatchString(line) {
			issues = append(issues, Issue{
				Line:       lineNum,
				Severity:   "error",
				Category:   "Logic Bug",
				Rule:       "NAN_COMPARISON",
				Message:    "Comparing directly with NaN is always false (NaN !== NaN).",
				Snippet:    line,
				Suggestion: "Use Number.isNaN(value) or isNaN(value) instead.",
			})
		}

		// Detect invalid typeof comparisons like typeof x === 'null' or 'int'
		if jsTypeofMistake.MatchString(line) {
			issues = append(issues, Issue{
				Line:       lineNum,
				Severity:   "error",
				Category:   "Logic Bug",
				Rule:       "TYPEOF_INVALID_COMPARISON",
				Message:    "Invalid typeof comparison target (typeof produces 'string', 'number', 'boolean', 'undefined', 'object', 'function', 'symbol', or 'bigint').",
				Snippet:    line,
				Suggestion: "Use valid typeof return values or Array.isArray() / strict null checks.",
			})
		}

		// Detect debugger statement
		if jsDebuggerRe.MatchString(line) {
			issues = append(issues, Issue{
				Line:       lineNum,
				Severity:   "warning",
				Category:   "Bad Practice",
				Rule:       "DEBUGGER_STATEMENT",
				Message:    "Debugger breakpoint statement left in code.",
				Snippet:    line,
				Suggestion: "Remove 'debugger;' statement before shipping.",
			})
		}

		// Detect 'with' statement
		if jsWithRe.MatchString(line) {
			issues = append(issues, Issue{
				Line:       lineNum,
				Severity:   "error",
				Category:   "Bad Practice",
				Rule:       "DISALLOWED_WITH_STATEMENT",
				Message:    "The 'with' statement is forbidden in JavaScript strict mode and causes unpredictable scoping.",
				Snippet:    line,
				Suggestion: "Access object properties directly or destructure needed properties.",
			})
		}

		// Check ASI pitfall: return on its own line followed by object or expression on next line
		if jsReturnNewline.MatchString(trimmed) && lineIdx+1 < len(lines) {
			nextLineTrim := strings.TrimSpace(lines[lineIdx+1])
			if strings.HasPrefix(nextLineTrim, "{") || strings.HasPrefix(nextLineTrim, "(") {
				issues = append(issues, Issue{
					Line:       lineNum,
					Severity:   "error",
					Category:   "Logic Bug",
					Rule:       "ASI_UNINTENDED_RETURN",
					Message:    "Automatic Semicolon Insertion (ASI) will insert a semicolon after 'return', returning undefined instead of the next line.",
					Snippet:    line,
					Suggestion: "Move the opening bracket/parenthesis to the same line: 'return (' or 'return {'.",
				})
			}
		}

		// Check unreachable code after return inside block
		if isAfterReturn {
			if strings.Contains(trimmed, "}") {
				isAfterReturn = false
			} else if !strings.HasPrefix(trimmed, "case ") && !strings.HasPrefix(trimmed, "default:") {
				issues = append(issues, Issue{
					Line:       lineNum,
					Severity:   "warning",
					Category:   "Dead Code",
					Rule:       "UNREACHABLE_CODE",
					Message:    fmt.Sprintf("Unreachable statement detected immediately following return statement on Line %d.", returnLine),
					Snippet:    line,
					Suggestion: "Remove or reposition code prior to the return statement.",
				})
				isAfterReturn = false
			}
		}

		if (strings.HasPrefix(trimmed, "return ") || strings.HasPrefix(trimmed, "throw ")) && strings.HasSuffix(trimmed, ";") {
			isAfterReturn = true
			returnLine = lineNum
		}
	}

	return issues
}
