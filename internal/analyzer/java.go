package analyzer

import (
	"fmt"
	"regexp"
	"strings"
)

var (
	javaStringEqualsRe   = regexp.MustCompile(`([a-zA-Z0-9_]+)\s*==\s*"[^"]*"|"[^"]*"\s*==\s*([a-zA-Z0-9_]+)`)
	javaMissingSemi      = regexp.MustCompile(`^\s*(public|private|protected|return|int|double|float|String|boolean|void)\b`)
	javaNullVarDecl      = regexp.MustCompile(`\b(?:String|Integer|Double|Boolean|List|Map|Set|Object|[A-Z][A-Za-z0-9_]*)\s+([a-zA-Z0-9_]+)\s*=\s*null\s*;`)
	javaAutoUnboxNull    = regexp.MustCompile(`\b(?:int|double|boolean|long)\s+([a-zA-Z0-9_]+)\s*=\s*([a-zA-Z0-9_]+)\s*;`)
	javaInvertedEquals   = regexp.MustCompile(`\b([a-zA-Z0-9_]+)\.equals\(\s*"[^"]+"\s*\)`)
)

// AnalyzeJava provides dedicated static analysis for Java, detecting NPE hazards, reference comparisons, and syntax errors.
func AnalyzeJava(lines []string) []Issue {
	var issues []Issue

	issues = append(issues, CheckLanguageBracketBalance(lines, "java")...)
	issues = append(issues, CheckCommonLogicalBugs(lines, "java")...)

	nullVars := make(map[string]int)

	for lineIdx, line := range lines {
		lineNum := lineIdx + 1
		trimmed := strings.TrimSpace(line)

		if trimmed == "" || strings.HasPrefix(trimmed, "//") || strings.HasPrefix(trimmed, "/*") || strings.HasPrefix(trimmed, "*") {
			continue
		}

		// --- Java NullPointerException (NPE) Detection ---
		if m := javaNullVarDecl.FindStringSubmatch(line); len(m) > 1 {
			nullVars[m[1]] = lineNum
		}
		for varName, declLine := range nullVars {
			// Check method invocation on null variable: varName.method()
			callPattern := regexp.MustCompile(`\b` + regexp.QuoteMeta(varName) + `\.[a-zA-Z0-9_]+\s*\(`)
			if callPattern.MatchString(line) && lineNum > declLine && !strings.Contains(line, varName+" = new") {
				issues = append(issues, Issue{
					Line:       lineNum,
					Severity:   "error",
					Category:   "NullPointerException Hazard",
					Rule:       "JAVA_NULL_POINTER_EXCEPTION",
					Message:    fmt.Sprintf("Calling method on '%s' will throw a NullPointerException (NPE); variable was initialized to null on Line %d.", varName, declLine),
					Snippet:    line,
					Suggestion: fmt.Sprintf("Initialize '%s' with a valid instance or wrap in 'if (%s != null)' check.", varName, varName),
				})
			}

			// Check auto-unboxing of null wrapper to primitive (e.g. int x = nullInteger)
			if m := javaAutoUnboxNull.FindStringSubmatch(line); len(m) > 2 && m[2] == varName && lineNum > declLine {
				issues = append(issues, Issue{
					Line:       lineNum,
					Severity:   "error",
					Category:   "NullPointerException Hazard",
					Rule:       "JAVA_NULL_AUTO_UNBOXING",
					Message:    fmt.Sprintf("Auto-unboxing null wrapper '%s' to primitive will throw NullPointerException at runtime.", varName),
					Snippet:    line,
					Suggestion: fmt.Sprintf("Verify that '%s' is not null before unboxing to primitive.", varName),
				})
			}
		}

		// String reference equality vs .equals()
		if javaStringEqualsRe.MatchString(line) {
			issues = append(issues, Issue{
				Line:       lineNum,
				Severity:   "warning",
				Category:   "Logic Bug",
				Rule:       "JAVA_STRING_EQUALITY_REF",
				Message:    "Comparing String using '==' tests object reference identity, not string content equality.",
				Snippet:    line,
				Suggestion: "Use '\"...\".equals(str)' or 'Objects.equals(a, b)' for value equality.",
			})
		}

		// Inverted equals: str.equals("literal") throws NPE if str is null; "literal".equals(str) is null-safe!
		if m := javaInvertedEquals.FindStringSubmatch(line); len(m) > 1 {
			varName := m[1]
			issues = append(issues, Issue{
				Line:       lineNum,
				Severity:   "info",
				Category:   "Null Safety Best Practice",
				Rule:       "JAVA_NULL_SAFE_LITERAL_EQUALS",
				Message:    fmt.Sprintf("Calling '%s.equals(\"...\")' will throw NullPointerException if '%s' is null.", varName, varName),
				Snippet:    line,
				Suggestion: fmt.Sprintf("Invert to '\"...\".equals(%s)' or 'Objects.equals(%s, \"...\")' to guarantee null safety.", varName, varName),
			})
		}

		// Missing semicolon in statement lines
		if javaMissingSemi.MatchString(trimmed) &&
			!strings.HasSuffix(trimmed, ";") && !strings.HasSuffix(trimmed, "{") && !strings.HasSuffix(trimmed, "}") &&
			!strings.Contains(trimmed, "abstract ") && !strings.Contains(trimmed, "interface ") && !strings.Contains(trimmed, "class ") {
			if strings.Contains(line, "=") || strings.Contains(line, "return ") || strings.Contains(line, "System.out.") {
				issues = append(issues, Issue{
					Line:       lineNum,
					Severity:   "error",
					Category:   "Syntax Error",
					Rule:       "JAVA_MISSING_SEMICOLON",
					Message:    "Missing terminating semicolon ';' at the end of Java statement.",
					Snippet:    line,
					Suggestion: "Add ';' to the end of the statement.",
				})
			}
		}
	}

	return deduplicateIssues(issues)
}
