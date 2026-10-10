package analyzer

import (
	"fmt"
	"regexp"
	"strings"
)

var (
	pyDefOrControlBlock  = regexp.MustCompile(`^\s*(if|elif|else|for|while|def|class|try|except|finally|with|async\s+def|async\s+with|async\s+for)\b`)
	pyPrintWithoutParens = regexp.MustCompile(`^\s*print\s+["'][^)]*$|^\s*print\s+[a-zA-Z0-9_]`)
	pyMutableDefaultArg  = regexp.MustCompile(`def\s+[a-zA-Z_]\w*\s*\([^)]*=\s*(\[\]|\{\})\s*[,)]`)
	pyIsComparison       = regexp.MustCompile(`\b\w+\s+(?:is|is\s+not)\s+(?:\d+|"[^"]*"|'[^']*')`)
	pyExceptAll          = regexp.MustCompile(`^\s*except\s*:`)
	pyEvalExec           = regexp.MustCompile(`\b(eval|exec)\s*\(`)
	pyEqNoneComparison   = regexp.MustCompile(`\b\w+\s*==\s*None\b|\bNone\s*==\s*\w+`)
	pyNeqNoneComparison  = regexp.MustCompile(`\b\w+\s*!=\s*None\b|\bNone\s*!=\s*\w+`)
	pyDirectNoneAccess   = regexp.MustCompile(`\bNone\.[a-zA-Z_]\w*|\bNone\[|\bNone\(`)
	pyAssignInplaceList  = regexp.MustCompile(`\b([a-zA-Z_]\w*)\s*=\s*[a-zA-Z_]\w*\.(append|sort|extend|reverse)\(`)
)

// AnalyzePython inspects Python code for syntax mistakes, bad practices, null/None bugs, and logical errors.
func AnalyzePython(lines []string) []Issue {
	var issues []Issue

	// Python-specific bracket check
	issues = append(issues, CheckLanguageBracketBalance(lines, "python")...)
	issues = append(issues, CheckCommonLogicalBugs(lines, "python")...)

	indentStack := []int{0}
	hasTabs := false
	hasSpaces := false

	// Track variables assigned to None
	assignedNone := make(map[string]int)

	for lineIdx, line := range lines {
		lineNum := lineIdx + 1
		trimmed := strings.TrimSpace(line)

		// Ignore empty lines or comments
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}

		// Calculate indentation
		leadingSpaces := 0
		leadingTabs := 0
		for _, ch := range line {
			if ch == ' ' {
				leadingSpaces++
			} else if ch == '\t' {
				leadingTabs++
			} else {
				break
			}
		}

		if leadingTabs > 0 {
			hasTabs = true
		}
		if leadingSpaces > 0 {
			hasSpaces = true
		}

		// Check mixed tabs and spaces
		if leadingTabs > 0 && leadingSpaces > 0 {
			issues = append(issues, Issue{
				Line:       lineNum,
				Severity:   "warning",
				Category:   "Indentation / Style",
				Rule:       "MIXED_TABS_AND_SPACES",
				Message:    "Mixed tabs and spaces in indentation causes TabError in Python.",
				Snippet:    line,
				Suggestion: "Use consistently 4 spaces for indentation.",
			})
		}

		// Check for missing colon after control statement / def / class
		if pyDefOrControlBlock.MatchString(trimmed) {
			codePart := trimmed
			if hashIdx := strings.Index(trimmed, "#"); hashIdx != -1 {
				codePart = strings.TrimSpace(trimmed[:hashIdx])
			}

			if !strings.HasSuffix(codePart, "\\") && !strings.HasSuffix(codePart, ":") {
				openParens := strings.Count(codePart, "(") - strings.Count(codePart, ")")
				if openParens <= 0 {
					issues = append(issues, Issue{
						Line:       lineNum,
						Severity:   "error",
						Category:   "Syntax Error",
						Rule:       "PYTHON_MISSING_COLON",
						Message:    fmt.Sprintf("Missing trailing colon ':' at the end of statement '%s'.", codePart),
						Snippet:    line,
						Suggestion: "Append a colon ':' to the end of the line.",
					})
				}
			}
		}

		// Check Python 2 print statement
		if pyPrintWithoutParens.MatchString(trimmed) {
			issues = append(issues, Issue{
				Line:       lineNum,
				Severity:   "error",
				Category:   "Syntax Error",
				Rule:       "PYTHON2_PRINT_SYNTAX",
				Message:    "Python 2 style print statement without parentheses is invalid in Python 3.",
				Snippet:    line,
				Suggestion: "Enclose print arguments in parentheses: print(...).",
			})
		}

		// Check Python off-by-one in range: range(len(arr) + 1)
		if strings.Contains(line, "range(len(") && strings.Contains(line, "+ 1)") {
			issues = append(issues, Issue{
				Line:       lineNum,
				Severity:   "warning",
				Category:   "Logic Bug",
				Rule:       "OFF_BY_ONE_RANGE",
				Message:    "Loop range 'range(len(...) + 1)' exceeds list bounds and will cause IndexError when indexed.",
				Snippet:    line,
				Suggestion: "Iterate over 'range(len(...))' or directly over elements with 'for item in list:'.",
			})
		}

		// --- Python Null / NoneType Hazards ---
		// Direct attribute or item access on None
		if pyDirectNoneAccess.MatchString(line) {
			issues = append(issues, Issue{
				Line:       lineNum,
				Severity:   "error",
				Category:   "Null / NoneType Crash Hazard",
				Rule:       "PYTHON_NONE_DEREFERENCE",
				Message:    "Attempted attribute access, subscript, or invocation on literal 'None' (AttributeError: 'NoneType' object has no attribute).",
				Snippet:    line,
				Suggestion: "Check for None before accessing properties or assign a valid instance.",
			})
		}

		// Track assignments to None: var = None
		if match := regexp.MustCompile(`^\s*([a-zA-Z_]\w*)\s*=\s*None\s*(?:#.*)?$`).FindStringSubmatch(line); len(match) > 1 {
			assignedNone[match[1]] = lineNum
		} else {
			// Check if a previously None variable is dereferenced without reassignment
			for varName, defLine := range assignedNone {
				derefPattern := regexp.MustCompile(`\b` + regexp.QuoteMeta(varName) + `\.[a-zA-Z_]\w*|\b` + regexp.QuoteMeta(varName) + `\[`)
				if derefPattern.MatchString(line) && lineNum > defLine && !strings.Contains(line, varName+" =") {
					issues = append(issues, Issue{
						Line:       lineNum,
						Severity:   "error",
						Category:   "Null / NoneType Crash Hazard",
						Rule:       "PYTHON_NULL_VARIABLE_DEREFERENCE",
						Message:    fmt.Sprintf("Variable '%s' was set to None on Line %d and accessed here without validation (AttributeError / TypeError).", varName, defLine),
						Snippet:    line,
						Suggestion: fmt.Sprintf("Add 'if %s is not None:' check prior to accessing attributes or subscript.", varName),
					})
				}
			}
		}

		// Comparing with None using == instead of 'is'
		if pyEqNoneComparison.MatchString(line) {
			issues = append(issues, Issue{
				Line:       lineNum,
				Severity:   "warning",
				Category:   "Null Safety / Style",
				Rule:       "PYTHON_EQ_NONE_COMPARISON",
				Message:    "Comparison to None using '==' violates PEP 8 and can trigger unintended custom __eq__ behaviors.",
				Snippet:    line,
				Suggestion: "Use 'is None' for identity comparison with singleton None.",
			})
		}
		if pyNeqNoneComparison.MatchString(line) {
			issues = append(issues, Issue{
				Line:       lineNum,
				Severity:   "warning",
				Category:   "Null Safety / Style",
				Rule:       "PYTHON_NEQ_NONE_COMPARISON",
				Message:    "Comparison to None using '!=' violates PEP 8.",
				Snippet:    line,
				Suggestion: "Use 'is not None' for identity comparison.",
			})
		}

		// List mutating in-place methods assigned to variable: x = lst.sort() -> x is None
		if pyAssignInplaceList.MatchString(line) {
			issues = append(issues, Issue{
				Line:       lineNum,
				Severity:   "warning",
				Category:   "Null / NoneType Hazard",
				Rule:       "PYTHON_INPLACE_METHOD_ASSIGNMENT",
				Message:    "List mutating method returns None; assigning it overwrites the variable with None.",
				Snippet:    line,
				Suggestion: "Call the method in place without assigning, or use sorted() instead of .sort().",
			})
		}

		// Check mutable default arguments: def func(a, b=[])
		if pyMutableDefaultArg.MatchString(line) {
			issues = append(issues, Issue{
				Line:       lineNum,
				Severity:   "warning",
				Category:   "Logic Bug",
				Rule:       "MUTABLE_DEFAULT_ARG",
				Message:    "Mutable default argument (list '[]' or dict '{}') is retained across function calls.",
				Snippet:    line,
				Suggestion: "Use 'None' as default value and initialize inside: 'if b is None: b = []'.",
			})
		}

		// Check 'is' with literals: x is 100 or x is 'str'
		if pyIsComparison.MatchString(line) {
			issues = append(issues, Issue{
				Line:       lineNum,
				Severity:   "warning",
				Category:   "Logic Bug",
				Rule:       "LITERAL_IS_COMPARISON",
				Message:    "Comparing literal with 'is' identity operator instead of equality '=='.",
				Snippet:    line,
				Suggestion: "Use '==' for value equality comparison instead of 'is'.",
			})
		}

		// Check bare except:
		if pyExceptAll.MatchString(trimmed) {
			issues = append(issues, Issue{
				Line:       lineNum,
				Severity:   "warning",
				Category:   "Bad Practice",
				Rule:       "BARE_EXCEPT",
				Message:    "Bare 'except:' catches KeyboardInterrupt and SystemExit, preventing graceful exit.",
				Snippet:    line,
				Suggestion: "Use 'except Exception:' or catch specific exception types.",
			})
		}

		// Check dangerous eval / exec
		if pyEvalExec.MatchString(line) {
			issues = append(issues, Issue{
				Line:       lineNum,
				Severity:   "warning",
				Category:   "Security Vulnerability",
				Rule:       "DANGEROUS_EVAL_EXEC",
				Message:    "Use of eval() / exec() exposes arbitrary code execution vulnerabilities.",
				Snippet:    line,
				Suggestion: "Use safer alternatives like ast.literal_eval() or json.loads().",
			})
		}
	}

	if hasTabs && hasSpaces {
		issues = append(issues, Issue{
			Line:       1,
			Severity:   "warning",
			Category:   "Indentation / Style",
			Rule:       "GLOBAL_INCONSISTENT_INDENT",
			Message:    "Document contains both tabs and spaces for indentation.",
			Snippet:    lines[0],
			Suggestion: "Convert all indentation to 4 spaces per indent level.",
		})
	}

	_ = indentStack
	return issues
}
