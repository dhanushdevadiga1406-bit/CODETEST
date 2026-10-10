package analyzer
import (
	"regexp"
	"sort"
	"strings"
	"time"
)

var pythonImportLine = regexp.MustCompile(`(?m)^\s*(?:from\s+[A-Za-z_][A-Za-z0-9_.]*\s+import\b|import\s+[A-Za-z_][A-Za-z0-9_.]*(?:\s*,\s*[A-Za-z_][A-Za-z0-9_.]*)*)\s*(?:#.*)?$`)

// DetectLanguage inspects code lines to identify the programming language with precision.
func DetectLanguage(code string) string {
	lower := strings.ToLower(code)

	// PHP
	if strings.Contains(code, "<?php") || strings.Contains(code, "$_SERVER") || strings.Contains(code, "$_POST") {
		return "php"
	}

	// SQL
	if (strings.Contains(lower, "select ") && strings.Contains(lower, " from ")) ||
		strings.Contains(lower, "create table ") ||
		strings.Contains(lower, "insert into ") ||
		strings.Contains(lower, "delete from ") {
		return "sql"
	}

	// Rust
	if strings.Contains(code, "fn main()") || strings.Contains(code, "let mut ") || strings.Contains(code, "println!(\"") || strings.Contains(code, "impl ") {
		return "rust"
	}

	// Swift
	if strings.Contains(code, "import Foundation") || strings.Contains(code, "import UIKit") || strings.Contains(code, "import SwiftUI") {
		return "swift"
	}

	// Kotlin
	if strings.Contains(code, "fun main(") || (strings.Contains(lower, "package ") && strings.Contains(code, "fun ")) {
		return "kotlin"
	}

	// Ruby
	if (strings.Contains(code, "def ") && strings.Contains(code, "end")) || strings.Contains(code, "puts ") || strings.Contains(code, "require '") {
		return "ruby"
	}

	// Go
	if strings.Contains(lower, "package ") && (strings.Contains(lower, "func ") || strings.Contains(lower, "import (")) {
		return "go"
	}

	// Python
	if strings.Contains(lower, "def ") || strings.Contains(lower, "elif ") || pythonImportLine.MatchString(code) {
		return "python"
	}

	// C# (.NET)
	if strings.Contains(lower, "console.writeline(") || strings.Contains(lower, "using system;") || strings.Contains(lower, "namespace ") && strings.Contains(lower, "class ") {
		return "csharp"
	}

	// Java
	if strings.Contains(lower, "public class ") ||
		strings.Contains(lower, "public interface ") ||
		strings.Contains(lower, "public enum ") ||
		strings.Contains(lower, "public record ") ||
		strings.Contains(lower, "system.out.println") {
		return "java"
	}

	// TypeScript
	if strings.Contains(lower, "interface ") && strings.Contains(lower, ":") ||
		strings.Contains(lower, "type ") && strings.Contains(lower, ":") {
		return "typescript"
	}

	// C++
	if strings.Contains(lower, "std::") ||
		strings.Contains(lower, "#include <iostream>") ||
		strings.Contains(lower, "#include <vector>") ||
		strings.Contains(lower, "#include <string>") ||
		strings.Contains(lower, "using namespace std") ||
		strings.Contains(lower, "template<") {
		return "cpp"
	}

	// C
	if strings.Contains(lower, "#include <stdio.h>") || strings.Contains(lower, "#include <stdlib.h>") ||
		(strings.Contains(lower, "#include") && !strings.Contains(lower, "std::")) ||
		strings.Contains(lower, "int main(") {
		return "c"
	}

	// JavaScript
	if strings.Contains(lower, "function ") || strings.Contains(lower, "console.log") || strings.Contains(lower, "const ") || strings.Contains(lower, "let ") {
		return "javascript"
	}

	// HTML / CSS
	if strings.Contains(lower, "<html") || strings.Contains(lower, "<!doctype") || strings.Contains(lower, "<div") {
		return "html"
	}

	return "generic"
}

// Analyze processes the given code and language using dedicated per-language analyzers.
func Analyze(req AnalyzeRequest) AnalyzeResponse {
	startTime := time.Now()

	lang := strings.ToLower(strings.TrimSpace(req.Language))
	if lang == "" || lang == "auto" {
		lang = DetectLanguage(req.Code)
	}

	// --- Null / Empty Application Check ---
	if strings.TrimSpace(req.Code) == "" {
		issues := []Issue{
			{
				Line:       1,
				Column:     1,
				Severity:   "error",
				Category:   "Null / Empty Application",
				Rule:       "NULL_OR_EMPTY_APPLICATION",
				Message:    "Null / Empty Application detected: The submitted application contains no executable code or statements.",
				Snippet:    "<empty / null application>",
				Suggestion: "Provide source code statements, classes, or a main entrypoint function for this application.",
			},
		}
		elapsed := float64(time.Since(startTime).Microseconds()) / 1000.0
		return AnalyzeResponse{
			Success:    true,
			Language:   lang,
			TotalLines: 0,
			Issues:     issues,
			Summary: Summary{
				TotalErrors:     1,
				TotalWarnings:   0,
				TotalInfos:      0,
				ExecutionTimeMs: elapsed,
				Status:          "ERRORS_FOUND",
			},
		}
	}

	// Split lines cleanly preserving empty lines inside the code
	cleanCode := strings.ReplaceAll(req.Code, "\r\n", "\n")
	if strings.HasSuffix(cleanCode, "\n") {
		cleanCode = strings.TrimSuffix(cleanCode, "\n")
	}
	lines := strings.Split(cleanCode, "\n")
	totalLines := len(lines)

	var issues []Issue

	// Dedicated per-language analyzers (Never use one language's rules for another!)
	switch lang {
	case "python", "py":
		issues = AnalyzePython(lines)
		issues = append(issues, ValidateWithToolchain(req.Code, "python")...)

	case "javascript", "js":
		issues = AnalyzeJavaScript(lines)
		issues = append(issues, ValidateWithToolchain(req.Code, "javascript")...)

	case "typescript", "ts":
		issues = AnalyzeJavaScript(lines)
		issues = append(issues, ValidateWithToolchain(req.Code, "typescript")...)

	case "go", "golang":
		issues = AnalyzeGolang(lines)
		issues = append(issues, ValidateWithToolchain(req.Code, "go")...)

	case "c":
		issues = AnalyzeC(lines)
		issues = append(issues, ValidateWithToolchain(req.Code, "c")...)

	case "cpp", "c++":
		issues = AnalyzeCpp(lines)
		issues = append(issues, ValidateWithToolchain(req.Code, "cpp")...)

	case "java":
		issues = AnalyzeJava(lines)
		issues = append(issues, ValidateWithToolchain(req.Code, "java")...)

	case "csharp", "c#", "cs":
		issues = AnalyzeCSharp(lines)
		issues = append(issues, ValidateWithToolchain(req.Code, "csharp")...)

	case "rust", "rs":
		issues = AnalyzeRust(lines)

	case "php":
		issues = AnalyzePHP(lines)

	case "ruby", "rb":
		issues = AnalyzeRuby(lines)

	case "kotlin", "kt":
		issues = AnalyzeKotlin(lines)

	case "swift":
		issues = AnalyzeSwift(lines)

	case "sql":
		issues = AnalyzeSQL(lines)

	case "html", "htm", "css":
		issues = AnalyzeHTMLCSS(lines)

	default:
		issues = append(issues, CheckLanguageBracketBalance(lines, "generic")...)
		issues = append(issues, CheckCommonLogicalBugs(lines, "generic")...)
	}

	// Ensure line numbers are within valid range and snippet is present
	for i := range issues {
		if issues[i].Line <= 0 {
			issues[i].Line = 1
		}
		if issues[i].Snippet == "" && issues[i].Line-1 < len(lines) {
			issues[i].Snippet = lines[issues[i].Line-1]
		}
	}

	// Sort issues by line number ascending, then column
	sort.Slice(issues, func(i, j int) bool {
		if issues[i].Line == issues[j].Line {
			return issues[i].Column < issues[j].Column
		}
		return issues[i].Line < issues[j].Line
	})

	// Calculate summary stats
	var totalErrors, totalWarnings, totalInfos int
	for _, issue := range issues {
		switch strings.ToLower(issue.Severity) {
		case "error":
			totalErrors++
		case "warning":
			totalWarnings++
		default:
			totalInfos++
		}
	}

	status := "PASSED"
	if totalErrors > 0 {
		status = "ERRORS_FOUND"
	} else if totalWarnings > 0 {
		status = "WARNINGS_FOUND"
	}

	elapsed := float64(time.Since(startTime).Microseconds()) / 1000.0
	if issues == nil {
		issues = []Issue{}
	}

	return AnalyzeResponse{
		Success:    true,
		Language:   lang,
		TotalLines: totalLines,
		Issues:     issues,
		Summary: Summary{
			TotalErrors:     totalErrors,
			TotalWarnings:   totalWarnings,
			TotalInfos:      totalInfos,
			ExecutionTimeMs: elapsed,
			Status:          status,
		},
	}
}
