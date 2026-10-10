package analyzer

import (
	"fmt"
	"go/parser"
	"go/scanner"
	"go/token"
	"regexp"
	"strings"
)

var (
	goMissingPackage   = regexp.MustCompile(`^\s*package\s+[a-zA-Z_]\w*`)
	goWaitGroupAdd     = regexp.MustCompile(`go\s+func\(.*\)\s*\{[^}]*wg\.Add\(`)
	goErrIgnored       = regexp.MustCompile(`\b([a-zA-Z0-9_]+)\s*,\s*_\s*:=\s*`)
	goNilMapVar        = regexp.MustCompile(`^\s*var\s+([a-zA-Z0-9_]+)\s+map\[`)
	goNilPtrVar        = regexp.MustCompile(`^\s*var\s+([a-zA-Z0-9_]+)\s+\*[a-zA-Z0-9_]+`)
	goNilChannelVar    = regexp.MustCompile(`^\s*var\s+([a-zA-Z0-9_]+)\s+chan\s+[a-zA-Z0-9_]+`)
	goRespDeferBeforeErr = regexp.MustCompile(`resp,\s*err\s*:=`)
)

// AnalyzeGolang parses Go code using Go's official AST parser, null/nil safety rules, and concurrency checks.
func AnalyzeGolang(lines []string) []Issue {
	var issues []Issue
	fullCode := strings.Join(lines, "\n")

	// Use official Go AST parser for exact syntax errors
	fset := token.NewFileSet()
	_, err := parser.ParseFile(fset, "input.go", fullCode, parser.AllErrors)
	if err != nil {
		if scannerErrors, ok := err.(scanner.ErrorList); ok {
			for _, scanErr := range scannerErrors {
				snippet := ""
				if scanErr.Pos.Line-1 >= 0 && scanErr.Pos.Line-1 < len(lines) {
					snippet = lines[scanErr.Pos.Line-1]
				}
				issues = append(issues, Issue{
					Line:       scanErr.Pos.Line,
					Column:     scanErr.Pos.Column,
					Severity:   "error",
					Category:   "Syntax Error",
					Rule:       "GO_SYNTAX_ERROR",
					Message:    scanErr.Msg,
					Snippet:    snippet,
					Suggestion: "Fix compiler syntax error at indicated column.",
				})
			}
		} else {
			issues = append(issues, Issue{
				Line:       1,
				Severity:   "error",
				Category:   "Syntax Error",
				Rule:       "GO_SYNTAX_ERROR",
				Message:    err.Error(),
				Snippet:    lines[0],
				Suggestion: "Check Go package syntax and imports.",
			})
		}
	}

	// Go-specific bracket and logic checks
	issues = append(issues, CheckLanguageBracketBalance(lines, "go")...)
	issues = append(issues, CheckCommonLogicalBugs(lines, "go")...)

	// Check if package statement exists
	hasPackage := false
	for _, l := range lines {
		if goMissingPackage.MatchString(l) {
			hasPackage = true
			break
		}
	}
	if !hasPackage {
		issues = append(issues, Issue{
			Line:       1,
			Severity:   "error",
			Category:   "Syntax Error",
			Rule:       "GO_MISSING_PACKAGE",
			Message:    "Missing 'package <name>' statement at the beginning of Go source file.",
			Snippet:    lines[0],
			Suggestion: "Add 'package main' or package name as the first non-comment line.",
		})
	}

	// Track nil variables
	nilMaps := make(map[string]int)
	nilPtrs := make(map[string]int)
	nilChans := make(map[string]int)

	for lineIdx, line := range lines {
		lineNum := lineIdx + 1
		trimmed := strings.TrimSpace(line)

		// Check ignored error pattern: result, _ := os.Open(...)
		if goErrIgnored.MatchString(line) && (strings.Contains(line, "Open") || strings.Contains(line, "Dial") || strings.Contains(line, "Marshal") || strings.Contains(line, "Unmarshal")) {
			issues = append(issues, Issue{
				Line:       lineNum,
				Severity:   "warning",
				Category:   "Logic Bug",
				Rule:       "GO_IGNORED_ERROR",
				Message:    "Error return is ignored via blank identifier '_'. Critical errors may go undetected.",
				Snippet:    line,
				Suggestion: "Assign error to a variable and handle: 'if err != nil { return err }'.",
			})
		}

		// Check wg.Add() inside goroutine pattern
		if strings.Contains(trimmed, "wg.Add(") && lineIdx > 0 && strings.Contains(lines[lineIdx-1], "go func") {
			issues = append(issues, Issue{
				Line:       lineNum,
				Severity:   "error",
				Category:   "Concurrency Bug",
				Rule:       "GO_WG_ADD_IN_GOROUTINE",
				Message:    "sync.WaitGroup.Add() invoked inside spawned goroutine causes race condition.",
				Snippet:    line,
				Suggestion: "Call wg.Add(1) before launching the goroutine.",
			})
		}

		// Check time.Tick leak
		if strings.Contains(trimmed, "time.Tick(") {
			issues = append(issues, Issue{
				Line:       lineNum,
				Severity:   "warning",
				Category:   "Memory Leak",
				Rule:       "GO_TIME_TICK_LEAK",
				Message:    "time.Tick() creates a ticker that cannot be stopped, leaking GC memory unless program lives indefinitely.",
				Snippet:    line,
				Suggestion: "Use 'ticker := time.NewTicker(...)' and 'defer ticker.Stop()'.",
			})
		}

		// --- Go Nil Safety Checks ---
		// 1. var m map[string]int without make()
		if m := goNilMapVar.FindStringSubmatch(line); len(m) > 1 {
			nilMaps[m[1]] = lineNum
		}
		for mapName, declLine := range nilMaps {
			writePattern := regexp.MustCompile(`\b` + regexp.QuoteMeta(mapName) + `\[[^\]]+\]\s*=`)
			if writePattern.MatchString(line) && lineNum > declLine && !strings.Contains(line, "make(map") {
				issues = append(issues, Issue{
					Line:       lineNum,
					Severity:   "error",
					Category:   "Nil Pointer Crash Hazard",
					Rule:       "GO_NIL_MAP_WRITE",
					Message:    fmt.Sprintf("Writing to uninitialized nil map '%s' (declared Line %d) causes fatal runtime panic.", mapName, declLine),
					Snippet:    line,
					Suggestion: fmt.Sprintf("Initialize map with make(): '%s = make(%s)'", mapName, "map[...]..."),
				})
			}
		}

		// 2. var ptr *Type without allocation
		if m := goNilPtrVar.FindStringSubmatch(line); len(m) > 1 {
			nilPtrs[m[1]] = lineNum
		}
		for ptrName, declLine := range nilPtrs {
			derefPattern := regexp.MustCompile(`\*\s*` + regexp.QuoteMeta(ptrName) + `\s*=|` + regexp.QuoteMeta(ptrName) + `\.[A-Za-z_]\w*`)
			if derefPattern.MatchString(line) && lineNum > declLine && !strings.Contains(line, ptrName+" = &") && !strings.Contains(line, "new(") {
				issues = append(issues, Issue{
					Line:       lineNum,
					Severity:   "error",
					Category:   "Nil Pointer Crash Hazard",
					Rule:       "GO_NIL_POINTER_DEREFERENCE",
					Message:    fmt.Sprintf("Dereferencing uninitialized nil pointer '%s' (declared Line %d) causes runtime panic: invalid memory address.", ptrName, declLine),
					Snippet:    line,
					Suggestion: fmt.Sprintf("Allocate pointer memory with new() or address-of operator '&' before dereferencing '%s'.", ptrName),
				})
			}
		}

		// 3. var ch chan int without make()
		if m := goNilChannelVar.FindStringSubmatch(line); len(m) > 1 {
			nilChans[m[1]] = lineNum
		}
		for chanName, declLine := range nilChans {
			sendRecvPattern := regexp.MustCompile(`\b` + regexp.QuoteMeta(chanName) + `\s*<-|<-` + regexp.QuoteMeta(chanName))
			if sendRecvPattern.MatchString(line) && lineNum > declLine && !strings.Contains(line, "make(chan") {
				issues = append(issues, Issue{
					Line:       lineNum,
					Severity:   "error",
					Category:   "Concurrency Deadlock Hazard",
					Rule:       "GO_NIL_CHANNEL_BLOCK",
					Message:    fmt.Sprintf("Sending or receiving on uninitialized nil channel '%s' blocks forever causing permanent deadlock.", chanName),
					Snippet:    line,
					Suggestion: fmt.Sprintf("Initialize channel with make(): '%s = make(chan ...)'", chanName),
				})
			}
		}

		// 4. Defer Body.Close() before checking err != nil
		if goRespDeferBeforeErr.MatchString(line) && lineIdx+1 < len(lines) && strings.Contains(lines[lineIdx+1], "defer resp.Body.Close()") {
			issues = append(issues, Issue{
				Line:       lineNum + 1,
				Severity:   "error",
				Category:   "Nil Pointer Crash Hazard",
				Rule:       "GO_NIL_HTTP_RESPONSE_DEFER",
				Message:    "Calling defer resp.Body.Close() before checking 'if err != nil' causes panic when resp is nil on network failure.",
				Snippet:    lines[lineIdx+1],
				Suggestion: "Check 'if err != nil { return err }' BEFORE calling resp.Body.Close().",
			})
		}
	}

	return deduplicateIssues(issues)
}

func deduplicateIssues(issues []Issue) []Issue {
	seen := make(map[string]bool)
	var result []Issue
	for _, issue := range issues {
		key := fmt.Sprintf("%d:%s:%s", issue.Line, issue.Rule, issue.Message)
		if !seen[key] {
			seen[key] = true
			result = append(result, issue)
		}
	}
	return result
}
