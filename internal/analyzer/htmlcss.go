package analyzer

import (
	"regexp"
	"strings"
)

var (
	htmlUnclosedTagRe = regexp.MustCompile(`<([a-zA-Z1-6]+)(?:\s+[^>]*)?>`)
	htmlCloseTagRe    = regexp.MustCompile(`</([a-zA-Z1-6]+)>`)
)

// AnalyzeHTMLCSS provides dedicated static analysis for HTML and CSS documents.
func AnalyzeHTMLCSS(lines []string) []Issue {
	var issues []Issue

	issues = append(issues, CheckLanguageBracketBalance(lines, "html")...)

	fullDoc := strings.Join(lines, "\n")
	lowerDoc := strings.ToLower(fullDoc)

	if strings.Contains(lowerDoc, "<html") && !strings.Contains(lowerDoc, "<!doctype") {
		issues = append(issues, Issue{
			Line:       1,
			Severity:   "warning",
			Category:   "HTML Standard",
			Rule:       "HTML_MISSING_DOCTYPE",
			Message:    "Missing '<!DOCTYPE html>' declaration at the beginning of document.",
			Snippet:    lines[0],
			Suggestion: "Add '<!DOCTYPE html>' as the first line to ensure standards-compliant rendering.",
		})
	}

	for lineIdx, line := range lines {
		lineNum := lineIdx + 1
		trimmed := strings.TrimSpace(line)

		if strings.HasPrefix(trimmed, "<!--") {
			continue
		}

		if strings.Contains(line, "<img") && !strings.Contains(line, "alt=") {
			issues = append(issues, Issue{
				Line:       lineNum,
				Severity:   "warning",
				Category:   "Accessibility (A11y)",
				Rule:       "HTML_IMG_MISSING_ALT",
				Message:    "<img> element is missing required 'alt' attribute for screen reader accessibility.",
				Snippet:    line,
				Suggestion: "Add an 'alt=\"...\"' description to the <img> tag.",
			})
		}
	}

	return deduplicateIssues(issues)
}
