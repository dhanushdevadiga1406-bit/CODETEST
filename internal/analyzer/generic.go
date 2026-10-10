package analyzer

import (
	"fmt"
	"regexp"
	"strings"
)

// BracketStackEntry tracks open brackets for syntax validation.
type BracketStackEntry struct {
	Char   rune
	Line   int
	Column int
}

// CheckBracketBalance performs bracket balancing across any source code with language awareness.
func CheckBracketBalance(lines []string) []Issue {
	return CheckLanguageBracketBalance(lines, "generic")
}

// CheckLanguageBracketBalance performs bracket balancing tailored to each language's comment and string rules.
func CheckLanguageBracketBalance(lines []string, language string) []Issue {
	var issues []Issue
	var stack []BracketStackEntry

	lang := strings.ToLower(language)
	isHashCommentLang := lang == "python" || lang == "py" || lang == "ruby" || lang == "rb" || lang == "shell" || lang == "sh" || lang == "bash"
	isSqlLang := lang == "sql"
	isHtmlLang := lang == "html" || lang == "xml"

	inMultiLineComment := false
	inMultiLineString := false
	multiLineQuoteChar := rune(0)

	for lineIdx, line := range lines {
		lineNum := lineIdx + 1
		runes := []rune(line)
		n := len(runes)

		inSingleQuote := false
		inDoubleQuote := false
		isEscaped := false

		for i := 0; i < n; i++ {
			ch := runes[i]

			// HTML comment <!-- ... -->
			if isHtmlLang {
				if inMultiLineComment {
					if ch == '-' && i+2 < n && runes[i+1] == '-' && runes[i+2] == '>' {
						inMultiLineComment = false
						i += 2
					}
					continue
				}
				if ch == '<' && i+3 < n && runes[i+1] == '!' && runes[i+2] == '-' && runes[i+3] == '-' {
					inMultiLineComment = true
					i += 3
					continue
				}
			}

			// Handle multi-line block comments /* ... */
			if inMultiLineComment {
				if ch == '*' && i+1 < n && runes[i+1] == '/' {
					inMultiLineComment = false
					i++
				}
				continue
			}

			// Handle multi-line strings (triple quotes `"""` or `'''` or backtick)
			if inMultiLineString {
				if multiLineQuoteChar == '`' && ch == '`' {
					inMultiLineString = false
				} else if (multiLineQuoteChar == '"' || multiLineQuoteChar == '\'') &&
					i+2 < n && runes[i] == multiLineQuoteChar && runes[i+1] == multiLineQuoteChar && runes[i+2] == multiLineQuoteChar {
					inMultiLineString = false
					i += 2
				}
				continue
			}

			// Check start of multi-line comment /*
			if !inSingleQuote && !inDoubleQuote && ch == '/' && i+1 < n && runes[i+1] == '*' {
				inMultiLineComment = true
				i++
				continue
			}

			// Check single-line comments:
			// // for C/C++/Java/JS/TS/Go/C#/Rust/PHP/Kotlin/Swift
			// # for Python/Ruby/Shell
			// -- for SQL
			if !inSingleQuote && !inDoubleQuote {
				if ch == '/' && i+1 < n && runes[i+1] == '/' {
					break // C-style line comment
				}
				if isHashCommentLang && ch == '#' {
					break // Script line comment
				}
				if isSqlLang && ch == '-' && i+1 < n && runes[i+1] == '-' {
					break // SQL line comment
				}
			}

			// Check triple quotes for Python
			if isHashCommentLang && !inSingleQuote && !inDoubleQuote && i+2 < n &&
				((ch == '"' && runes[i+1] == '"' && runes[i+2] == '"') || (ch == '\'' && runes[i+1] == '\'' && runes[i+2] == '\'')) {
				inMultiLineString = true
				multiLineQuoteChar = ch
				i += 2
				continue
			}

			// Check backtick `
			if !inSingleQuote && !inDoubleQuote && ch == '`' {
				inMultiLineString = true
				multiLineQuoteChar = '`'
				continue
			}

			// Handle escape sequence
			if isEscaped {
				isEscaped = false
				continue
			}
			if ch == '\\' {
				isEscaped = true
				continue
			}

			// Handle single/double strings
			if ch == '\'' && !inDoubleQuote {
				inSingleQuote = !inSingleQuote
				continue
			}
			if ch == '"' && !inSingleQuote {
				inDoubleQuote = !inDoubleQuote
				continue
			}

			if inSingleQuote || inDoubleQuote {
				continue
			}

			// Bracket matching
			switch ch {
			case '(', '[', '{':
				stack = append(stack, BracketStackEntry{Char: ch, Line: lineNum, Column: i + 1})
			case ')', ']', '}':
				expected := rune(0)
				switch ch {
				case ')':
					expected = '('
				case ']':
					expected = '['
				case '}':
					expected = '{'
				}

				if len(stack) == 0 {
					issues = append(issues, Issue{
						Line:       lineNum,
						Column:     i + 1,
						Severity:   "error",
						Category:   "Syntax Error",
						Rule:       "UNMATCHED_CLOSING_BRACKET",
						Message:    fmt.Sprintf("Unexpected closing bracket '%c' without a matching opening bracket.", ch),
						Snippet:    line,
						Suggestion: fmt.Sprintf("Remove '%c' or add matching '%c' before it.", ch, expected),
					})
				} else {
					top := stack[len(stack)-1]
					stack = stack[:len(stack)-1]
					if top.Char != expected {
						issues = append(issues, Issue{
							Line:       lineNum,
							Column:     i + 1,
							Severity:   "error",
							Category:   "Syntax Error",
							Rule:       "MISMATCHED_BRACKETS",
							Message:    fmt.Sprintf("Mismatched closing bracket '%c'; expected closing for '%c' opened at Line %d, Col %d.", ch, top.Char, top.Line, top.Column),
							Snippet:    line,
							Suggestion: fmt.Sprintf("Replace '%c' with appropriate matching bracket or inspect Line %d.", ch, top.Line),
						})
					}
				}
			}
		}

		if inSingleQuote {
			issues = append(issues, Issue{
				Line:       lineNum,
				Severity:   "error",
				Category:   "Syntax Error",
				Rule:       "UNCLOSED_STRING_LITERAL",
				Message:    "Unclosed single quote (') string literal before end of line.",
				Snippet:    line,
				Suggestion: "Add a closing single quote (') or use escape \\' if part of string.",
			})
		}
		if inDoubleQuote {
			issues = append(issues, Issue{
				Line:       lineNum,
				Severity:   "error",
				Category:   "Syntax Error",
				Rule:       "UNCLOSED_STRING_LITERAL",
				Message:    "Unclosed double quote (\") string literal before end of line.",
				Snippet:    line,
				Suggestion: "Add a closing double quote (\") or escape internal quotes.",
			})
		}
	}

	// Any unclosed brackets left on stack
	for _, entry := range stack {
		expectedClose := ')'
		if entry.Char == '[' {
			expectedClose = ']'
		} else if entry.Char == '{' {
			expectedClose = '}'
		}

		var snip string
		if entry.Line-1 >= 0 && entry.Line-1 < len(lines) {
			snip = lines[entry.Line-1]
		}
		issues = append(issues, Issue{
			Line:       entry.Line,
			Column:     entry.Column,
			Severity:   "error",
			Category:   "Syntax Error",
			Rule:       "UNCLOSED_OPEN_BRACKET",
			Message:    fmt.Sprintf("Unclosed opening bracket '%c'. Missing closing '%c'.", entry.Char, expectedClose),
			Snippet:    snip,
			Suggestion: fmt.Sprintf("Close with '%c' at the appropriate block ending.", expectedClose),
		})
	}

	return issues
}

var (
	divByZeroRe         = regexp.MustCompile(`(?:\b|[^\w])(?:/|%)\s*0(?:\.0*)?(?:\b|[^\w])`)
	divByZeroAssignRe   = regexp.MustCompile(`(?:/=|%=)\s*0(?:\.0*)?(?:\b|[^\w])`)
	assignInConditionRe = regexp.MustCompile(`\b(?:if|while)\s*\((?:[^=!<>]|\b)*[a-zA-Z_]\w*\s*=\s*[^=]`)
	infiniteLoopWhile1  = regexp.MustCompile(`\bwhile\s*\(\s*(?:true|1)\s*\)\s*\{([^}]*)\}`)
	emptyCatchRe        = regexp.MustCompile(`\b(?:catch|except)\s*(?:\([^)]*\)|[a-zA-Z0-9_]*\s*:)?\s*\{\s*\}|\bexcept.*:\s*(?:pass|\.\.\.)`)
	offByOneArrayBound  = regexp.MustCompile(`\[\s*(?:len\([^)]+\)|[a-zA-Z0-9_]+\.length|[a-zA-Z0-9_]+\.size\(\))\s*\]`)
)

// CheckCommonLogicalBugs runs regex heuristics for dangerous code bugs.
func CheckCommonLogicalBugs(lines []string, language string) []Issue {
	var issues []Issue

	fullCode := strings.Join(lines, "\n")
	lang := strings.ToLower(language)

	for lineIdx, line := range lines {
		lineNum := lineIdx + 1
		trimmed := strings.TrimSpace(line)

		// Skip full comment lines according to language
		if isFullLineComment(trimmed, lang) {
			continue
		}

		// Division by Zero check
		if divByZeroRe.MatchString(line) || divByZeroAssignRe.MatchString(line) {
			if !isCommented(line, "/ 0", lang) && !isCommented(line, "% 0", lang) {
				issues = append(issues, Issue{
					Line:       lineNum,
					Severity:   "error",
					Category:   "Runtime Crash Hazard",
					Rule:       "DIVISION_BY_ZERO",
					Message:    "Division or modulo by zero constant detected.",
					Snippet:    line,
					Suggestion: "Ensure the divisor is non-zero or add a validation check prior to division.",
				})
			}
		}

		// Assignment inside if/while condition (e.g. if (x = 10)) - C-style languages
		if lang != "python" && lang != "py" && assignInConditionRe.MatchString(line) {
			if !strings.Contains(line, "==") && !strings.Contains(line, "<=") && !strings.Contains(line, ">=") && !strings.Contains(line, "!=") && !strings.Contains(line, ":=") {
				issues = append(issues, Issue{
					Line:       lineNum,
					Severity:   "warning",
					Category:   "Logic Bug",
					Rule:       "ASSIGNMENT_IN_CONDITION",
					Message:    "Accidental assignment '=' inside condition expression. Did you mean comparison '=='?",
					Snippet:    line,
					Suggestion: "Use '==' for comparison or ':=' / explicit assignment before condition.",
				})
			}
		}

		// Array out of bounds (off-by-one): arr[len(arr)] or arr[arr.length] in 0-indexed languages
		if offByOneArrayBound.MatchString(line) {
			issues = append(issues, Issue{
				Line:       lineNum,
				Severity:   "error",
				Category:   "Runtime Crash Hazard",
				Rule:       "INDEX_OUT_OF_BOUNDS",
				Message:    "Accessing array at index equal to its length (0-indexed collection will panic or be undefined).",
				Snippet:    line,
				Suggestion: "Use [len - 1] or [length - 1] to access the last element.",
			})
		}

		// Empty catch block
		if emptyCatchRe.MatchString(line) {
			issues = append(issues, Issue{
				Line:       lineNum,
				Severity:   "info",
				Category:   "Bad Practice",
				Rule:       "SILENT_EXCEPTION_SWALLOWED",
				Message:    "Empty catch block silently swallows errors, hiding runtime exceptions.",
				Snippet:    line,
				Suggestion: "Log the caught error or re-throw it appropriately.",
			})
		}
	}

	// Multi-line infinite loop check (e.g. while(true) with no break)
	matches := infiniteLoopWhile1.FindAllStringSubmatchIndex(fullCode, -1)
	for _, m := range matches {
		if len(m) >= 4 {
			body := fullCode[m[2]:m[3]]
			if !strings.Contains(body, "break") && !strings.Contains(body, "return") && !strings.Contains(body, "throw") && !strings.Contains(body, "panic") {
				startLine := 1 + strings.Count(fullCode[:m[0]], "\n")
				issues = append(issues, Issue{
					Line:       startLine,
					Severity:   "warning",
					Category:   "Logic Bug",
					Rule:       "POTENTIAL_INFINITE_LOOP",
					Message:    "Infinite loop 'while(true)' detected with no 'break', 'return', or exit statement inside the body.",
					Snippet:    lines[startLine-1],
					Suggestion: "Add a terminating condition with 'break' or 'return' inside the loop.",
				})
			}
		}
	}

	return issues
}

func isFullLineComment(trimmed, lang string) bool {
	if strings.HasPrefix(trimmed, "//") || strings.HasPrefix(trimmed, "/*") || strings.HasPrefix(trimmed, "*") {
		return true
	}
	if (lang == "python" || lang == "py" || lang == "ruby" || lang == "rb" || lang == "shell" || lang == "bash") && strings.HasPrefix(trimmed, "#") {
		return true
	}
	if lang == "sql" && strings.HasPrefix(trimmed, "--") {
		return true
	}
	return false
}

func isCommented(line, sub, lang string) bool {
	idx := strings.Index(line, sub)
	if idx == -1 {
		return false
	}
	commentSlash := strings.Index(line, "//")
	if commentSlash != -1 && commentSlash < idx {
		return true
	}
	if (lang == "python" || lang == "py" || lang == "ruby" || lang == "rb" || lang == "shell" || lang == "bash") {
		commentHash := strings.Index(line, "#")
		if commentHash != -1 && commentHash < idx {
			return true
		}
	}
	if lang == "sql" {
		commentDash := strings.Index(line, "--")
		if commentDash != -1 && commentDash < idx {
			return true
		}
	}
	return false
}
