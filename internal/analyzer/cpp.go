package analyzer

import (
	"fmt"
	"regexp"
	"strings"
)

var (
	cppUnsafeStrcpy   = regexp.MustCompile(`\bstrcpy\s*\(`)
	cppUnsafeGets     = regexp.MustCompile(`\bgets\s*\(`)
	cppDoubleFree     = regexp.MustCompile(`\bfree\s*\(\s*([a-zA-Z0-9_]+)\s*\)`)
	cppNullptrDecl    = regexp.MustCompile(`\b(?:auto|\w+)\s*\*\s*([a-zA-Z0-9_]+)\s*=\s*nullptr\s*;`)
	cppArrayNewScalarDelete = regexp.MustCompile(`\bdelete\s+([a-zA-Z0-9_]+)\s*;`)
)

// AnalyzeCpp provides dedicated static analysis for C++ code, checking nullptr safety, RAII, and syntax.
func AnalyzeCpp(lines []string) []Issue {
	var issues []Issue

	issues = append(issues, CheckLanguageBracketBalance(lines, "cpp")...)
	issues = append(issues, CheckCommonLogicalBugs(lines, "cpp")...)

	freedPointers := make(map[string]int)
	nullptrVars := make(map[string]int)
	arrayNewVars := make(map[string]int)

	for lineIdx, line := range lines {
		lineNum := lineIdx + 1
		trimmed := strings.TrimSpace(line)

		if trimmed == "" || strings.HasPrefix(trimmed, "//") || strings.HasPrefix(trimmed, "/*") || strings.HasPrefix(trimmed, "#") {
			continue
		}

		// Track new[] allocations to detect delete vs delete[]
		if m := regexp.MustCompile(`\b([a-zA-Z0-9_]+)\s*=\s*new\s+\w+\[`).FindStringSubmatch(line); len(m) > 1 {
			arrayNewVars[m[1]] = lineNum
		}

		// Detect scalar delete on array-allocated pointer
		if m := cppArrayNewScalarDelete.FindStringSubmatch(line); len(m) > 1 {
			varName := m[1]
			if allocLine, isArr := arrayNewVars[varName]; isArr && !strings.Contains(line, "delete[]") {
				issues = append(issues, Issue{
					Line:       lineNum,
					Severity:   "error",
					Category:   "Memory Deallocation Hazard",
					Rule:       "CPP_MISMATCHED_ARRAY_DELETE",
					Message:    fmt.Sprintf("Pointer '%s' was allocated with 'new[]' on Line %d, but freed with scalar 'delete' instead of 'delete[]'.", varName, allocLine),
					Snippet:    line,
					Suggestion: fmt.Sprintf("Use 'delete[] %s;' to deallocate array memory.", varName),
				})
			}
		}

		// --- C++ nullptr Dereference Checks ---
		if m := cppNullptrDecl.FindStringSubmatch(line); len(m) > 1 {
			nullptrVars[m[1]] = lineNum
		}
		for ptrName, declLine := range nullptrVars {
			derefPattern := regexp.MustCompile(`\*\s*` + regexp.QuoteMeta(ptrName) + `\b|` + regexp.QuoteMeta(ptrName) + `\s*->|` + regexp.QuoteMeta(ptrName) + `\[`)
			if derefPattern.MatchString(line) && lineNum > declLine && !strings.Contains(line, ptrName+" = new") && !strings.Contains(line, ptrName+" = &") {
				issues = append(issues, Issue{
					Line:       lineNum,
					Severity:   "error",
					Category:   "nullptr Crash Hazard",
					Rule:       "CPP_NULLPTR_DEREFERENCE",
					Message:    fmt.Sprintf("Dereferencing pointer '%s' initialized to nullptr on Line %d causes crash / segmentation fault.", ptrName, declLine),
					Snippet:    line,
					Suggestion: fmt.Sprintf("Initialize with 'new' or add 'if (%s != nullptr)' check before dereferencing.", ptrName),
				})
			}
		}

		// Dangerous gets()
		if cppUnsafeGets.MatchString(line) {
			issues = append(issues, Issue{
				Line:       lineNum,
				Severity:   "error",
				Category:   "Security Vulnerability",
				Rule:       "CPP_DANGEROUS_GETS",
				Message:    "gets() does not check buffer boundaries and was removed in modern C++ standard.",
				Snippet:    line,
				Suggestion: "Use std::getline(std::cin, str) with std::string.",
			})
		}

		// Dangerous strcpy()
		if cppUnsafeStrcpy.MatchString(line) {
			issues = append(issues, Issue{
				Line:       lineNum,
				Severity:   "warning",
				Category:   "Security Vulnerability",
				Rule:       "CPP_UNSAFE_STRCPY",
				Message:    "strcpy() cannot prevent buffer overflow vulnerabilities.",
				Snippet:    line,
				Suggestion: "Use std::string or strncpy_s() for safe string manipulation.",
			})
		}

		// Double free detection
		matches := cppDoubleFree.FindAllStringSubmatch(line, -1)
		for _, m := range matches {
			if len(m) > 1 {
				ptr := m[1]
				if prevLine, ok := freedPointers[ptr]; ok {
					issues = append(issues, Issue{
						Line:       lineNum,
						Severity:   "error",
						Category:   "Runtime Crash Hazard",
						Rule:       "CPP_DOUBLE_FREE",
						Message:    fmt.Sprintf("Double free on pointer '%s' (already freed at Line %d).", ptr, prevLine),
						Snippet:    line,
						Suggestion: "Set pointer to nullptr immediately after freeing: 'free(p); p = nullptr;'.",
					})
				} else {
					freedPointers[ptr] = lineNum
				}
			}
		}

		// Missing semicolon detection
		if !strings.HasSuffix(trimmed, ";") && !strings.HasSuffix(trimmed, "{") && !strings.HasSuffix(trimmed, "}") &&
			!strings.HasSuffix(trimmed, ":") && !strings.HasSuffix(trimmed, "\\") && !strings.HasPrefix(trimmed, "if") &&
			!strings.HasPrefix(trimmed, "while") && !strings.HasPrefix(trimmed, "for") && !strings.HasPrefix(trimmed, "else") &&
			!strings.HasPrefix(trimmed, "#") {
			if strings.Contains(line, "=") || strings.Contains(line, "return ") || strings.Contains(line, "std::cout") || strings.Contains(line, "printf(") {
				issues = append(issues, Issue{
					Line:       lineNum,
					Severity:   "error",
					Category:   "Syntax Error",
					Rule:       "CPP_MISSING_SEMICOLON",
					Message:    "Missing terminating semicolon ';' at the end of statement.",
					Snippet:    line,
					Suggestion: "Add ';' to terminate the C++ statement.",
				})
			}
		}
	}

	return deduplicateIssues(issues)
}
