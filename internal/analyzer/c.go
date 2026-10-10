package analyzer

import (
	"fmt"
	"regexp"
	"strings"
)

var (
	cUnsafeGets         = regexp.MustCompile(`\bgets\s*\(`)
	cUnsafeStrcpy       = regexp.MustCompile(`\bstrcpy\s*\(`)
	cUnsafeSprintf      = regexp.MustCompile(`\bsprintf\s*\(`)
	cDoubleFree         = regexp.MustCompile(`\bfree\s*\(\s*([a-zA-Z0-9_]+)\s*\)`)
	cNullPointerDecl    = regexp.MustCompile(`\b([A-Za-z_]\w*)\s*=\s*(?:NULL|nullptr|\(void\s*\*\)0)\b`)
	cScanfMissingRef    = regexp.MustCompile(`\bscanf\s*\(\s*"%d"\s*,\s*([A-Za-z_]\w*)\s*\)`)
	cReturnStackAddr    = regexp.MustCompile(`\breturn\s+&([A-Za-z_]\w*)\s*;`)
)

// AnalyzeC provides dedicated, language-specific static analysis for the C programming language.
func AnalyzeC(lines []string) []Issue {
	var issues []Issue

	// C-specific bracket and logic checks
	issues = append(issues, CheckLanguageBracketBalance(lines, "c")...)
	issues = append(issues, CheckCommonLogicalBugs(lines, "c")...)

	freedPointers := make(map[string]int)
	nullPointers := make(map[string]int)
	var localVars []string

	for lineIdx, line := range lines {
		lineNum := lineIdx + 1
		trimmed := strings.TrimSpace(line)

		if trimmed == "" || strings.HasPrefix(trimmed, "//") || strings.HasPrefix(trimmed, "/*") ||
			strings.HasPrefix(trimmed, "*/") ||
			(strings.HasPrefix(trimmed, "* ") && !strings.ContainsAny(trimmed, "=;()")) {
			continue
		}

		// Track local variables to detect returning stack addresses
		if m := regexp.MustCompile(`^\s*(?:int|char|float|double|long|short)\s+([a-zA-Z_]\w*)\b`).FindStringSubmatch(line); len(m) > 1 {
			localVars = append(localVars, m[1])
		}

		// Returning address of local stack variable (dangling pointer)
		if m := cReturnStackAddr.FindStringSubmatch(line); len(m) > 1 {
			varName := m[1]
			for _, loc := range localVars {
				if loc == varName {
					issues = append(issues, Issue{
						Line:       lineNum,
						Severity:   "error",
						Category:   "Dangling Pointer Hazard",
						Rule:       "C_RETURN_STACK_ADDRESS",
						Message:    fmt.Sprintf("Returning address of stack-allocated local variable '&%s' causes undefined behavior.", varName),
						Snippet:    line,
						Suggestion: "Allocate dynamic memory using malloc() or pass an output buffer pointer as a parameter.",
					})
				}
			}
		}

		// Dangerous gets()
		if cUnsafeGets.MatchString(line) {
			issues = append(issues, Issue{
				Line:       lineNum,
				Severity:   "error",
				Category:   "Security Vulnerability",
				Rule:       "C_DANGEROUS_GETS",
				Message:    "gets() does not perform buffer boundary checks and is deprecated/removed in C11.",
				Snippet:    line,
				Suggestion: "Replace gets() with fgets(buffer, sizeof(buffer), stdin).",
			})
		}

		// Unsafe strcpy()
		if cUnsafeStrcpy.MatchString(line) {
			issues = append(issues, Issue{
				Line:       lineNum,
				Severity:   "warning",
				Category:   "Security Vulnerability",
				Rule:       "C_UNSAFE_STRCPY",
				Message:    "strcpy() does not prevent buffer overflow vulnerabilities.",
				Snippet:    line,
				Suggestion: "Use strncpy() or snprintf() with explicit buffer size bounds.",
			})
		}

		// Unsafe sprintf()
		if cUnsafeSprintf.MatchString(line) {
			issues = append(issues, Issue{
				Line:       lineNum,
				Severity:   "warning",
				Category:   "Security Vulnerability",
				Rule:       "C_UNSAFE_SPRINTF",
				Message:    "sprintf() does not bound output buffer length and is prone to buffer overflows.",
				Snippet:    line,
				Suggestion: "Use snprintf(buf, sizeof(buf), ...) to enforce maximum buffer length.",
			})
		}

		// --- C NULL Pointer Dereference Checks ---
		if m := cNullPointerDecl.FindStringSubmatch(line); len(m) > 1 {
			nullPointers[m[1]] = lineNum
		}
		for ptrName, declLine := range nullPointers {
			// Dereference: *ptr = ... or ptr->field or ptr[0]
			derefPattern := regexp.MustCompile(`\*\s*` + regexp.QuoteMeta(ptrName) + `\b|` + regexp.QuoteMeta(ptrName) + `\s*->|` + regexp.QuoteMeta(ptrName) + `\[`)
			if derefPattern.MatchString(line) && lineNum > declLine && !strings.Contains(line, ptrName+" = malloc") && !strings.Contains(line, ptrName+" = &") {
				issues = append(issues, Issue{
					Line:       lineNum,
					Severity:   "error",
					Category:   "NULL Pointer Crash Hazard",
					Rule:       "C_NULL_POINTER_DEREFERENCE",
					Message:    fmt.Sprintf("Dereferencing NULL pointer '%s' (initialized to NULL on Line %d) causes Segmentation Fault (SIGSEGV).", ptrName, declLine),
					Snippet:    line,
					Suggestion: fmt.Sprintf("Allocate memory via malloc() or check 'if (%s != NULL)' before dereferencing.", ptrName),
				})
			}
		}

		// Double free / Use after free detection
		matches := cDoubleFree.FindAllStringSubmatch(line, -1)
		for _, m := range matches {
			if len(m) > 1 {
				ptr := m[1]
				if prevLine, ok := freedPointers[ptr]; ok {
					issues = append(issues, Issue{
						Line:       lineNum,
						Severity:   "error",
						Category:   "Runtime Crash Hazard",
						Rule:       "C_DOUBLE_FREE",
						Message:    fmt.Sprintf("Double free on pointer '%s' (previously freed at Line %d) causes heap corruption.", ptr, prevLine),
						Snippet:    line,
						Suggestion: "Set pointer to NULL immediately after freeing: 'free(p); p = NULL;'.",
					})
				} else {
					freedPointers[ptr] = lineNum
				}
			}
		}

		// Use after free check
		for ptr, freeLine := range freedPointers {
			usePattern := regexp.MustCompile(`\*\s*` + regexp.QuoteMeta(ptr) + `\b|` + regexp.QuoteMeta(ptr) + `->|` + regexp.QuoteMeta(ptr) + `\[`)
			if usePattern.MatchString(line) && lineNum > freeLine && !strings.Contains(line, ptr+" =") {
				issues = append(issues, Issue{
					Line:       lineNum,
					Severity:   "error",
					Category:   "Use-After-Free Crash Hazard",
					Rule:       "C_USE_AFTER_FREE",
					Message:    fmt.Sprintf("Use-after-free: pointer '%s' was already deallocated on Line %d.", ptr, freeLine),
					Snippet:    line,
					Suggestion: "Do not dereference pointer after free(); assign to NULL immediately.",
				})
			}
		}

		// scanf without address-of operator &
		if m := cScanfMissingRef.FindStringSubmatch(line); len(m) > 1 {
			varName := m[1]
			issues = append(issues, Issue{
				Line:       lineNum,
				Severity:   "error",
				Category:   "Runtime Crash Hazard",
				Rule:       "C_SCANF_MISSING_ADDRESS_OF",
				Message:    fmt.Sprintf("scanf format '%%d' requires pointer to int, but received '%s' without address-of operator '&'.", varName),
				Snippet:    line,
				Suggestion: fmt.Sprintf("Pass address '&%s' to scanf: scanf(\"%%d\", &%s);", varName, varName),
			})
		}

		// Missing semicolon in statement
		if !strings.HasSuffix(trimmed, ";") && !strings.HasSuffix(trimmed, "{") && !strings.HasSuffix(trimmed, "}") &&
			!strings.HasSuffix(trimmed, ":") && !strings.HasSuffix(trimmed, "\\") && !strings.HasPrefix(trimmed, "if") &&
			!strings.HasPrefix(trimmed, "while") && !strings.HasPrefix(trimmed, "for") && !strings.HasPrefix(trimmed, "else") &&
			!strings.HasPrefix(trimmed, "#") {
			if strings.Contains(line, "=") || strings.Contains(line, "return ") || strings.Contains(line, "printf(") || strings.Contains(line, "free(") {
				issues = append(issues, Issue{
					Line:       lineNum,
					Severity:   "error",
					Category:   "Syntax Error",
					Rule:       "C_MISSING_SEMICOLON",
					Message:    "Missing terminating semicolon ';' at the end of statement in C source.",
					Snippet:    line,
					Suggestion: "Add ';' to terminate the C statement.",
				})
			}
		}
	}

	return deduplicateIssues(issues)
}
