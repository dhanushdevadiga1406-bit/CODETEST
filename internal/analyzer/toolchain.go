package analyzer

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"time"
)

const toolchainTimeout = 8 * time.Second
const extendedToolchainTimeout = 45 * time.Second

var (
	gccDiagnostic    = regexp.MustCompile(`^.*?:(\d+):(\d+):\s*(fatal error|error|warning|note):\s*(.*)$`)
	javacDiagnostic  = regexp.MustCompile(`^.*?:(\d+):\s*(error|warning):\s*(.*)$`)
	pythonFileLine   = regexp.MustCompile(`^\s*File ".*", line (\d+)`)
	javaPublicType   = regexp.MustCompile(`\bpublic\s+(?:(?:final|abstract|sealed|non-sealed)\s+)*(?:class|interface|enum|record)\s+([A-Za-z_$][A-Za-z0-9_$]*)`)
	nodeFileLine     = regexp.MustCompile(`^.*:(\d+)\s*$`)
	tscDiagnostic    = regexp.MustCompile(`^.*\((\d+),(\d+)\):\s*(error|warning)\s+TS\d+:\s*(.*)$`)
	dotnetDiagnostic = regexp.MustCompile(`^.*\((\d+),(\d+)\):\s*(error|warning)\s+([A-Z]+\d+):\s*(.*)$`)
	dotnetSDKVersion = regexp.MustCompile(`(?m)^(\d+)\.\d+\.\d+`)
	goDiagnostic     = regexp.MustCompile(`^.*\.go:(\d+):(\d+):\s*(.*)$`)
)

// ValidateWithToolchain performs syntax-only validation using the language's
// standard parser or compiler. It never runs the submitted program.
func ValidateWithToolchain(code, language string) []Issue {
	command, args, filename, unavailableMessage := toolchainCommand(language, code)
	if command == "" {
		return []Issue{toolchainUnavailableIssue(unavailableMessage)}
	}

	tempDir, err := os.MkdirTemp("", "codetest-analyze-*")
	if err != nil {
		return []Issue{toolchainFailureIssue("Unable to create a temporary directory for syntax validation: " + err.Error())}
	}
	defer os.RemoveAll(tempDir)

	sourcePath := filepath.Join(tempDir, filename)
	if err := os.WriteFile(sourcePath, []byte(code), 0600); err != nil {
		return []Issue{toolchainFailureIssue("Unable to write source for syntax validation: " + err.Error())}
	}
	if language == "csharp" || language == "c#" || language == "cs" {
		project := `<Project Sdk="Microsoft.NET.Sdk">
  <PropertyGroup>
    <OutputType>Library</OutputType>
    <TargetFramework>net8.0</TargetFramework>
    <ImplicitUsings>disable</ImplicitUsings>
    <Nullable>disable</Nullable>
  </PropertyGroup>
</Project>
`
		if err := os.WriteFile(filepath.Join(tempDir, "CODETEST.csproj"), []byte(project), 0600); err != nil {
			return []Issue{toolchainFailureIssue("Unable to create temporary .NET project for syntax validation: " + err.Error())}
		}
	}

	for i := range args {
		args[i] = strings.ReplaceAll(args[i], "{source}", sourcePath)
		args[i] = strings.ReplaceAll(args[i], "{temp}", tempDir)
	}

	timeout := toolchainTimeout
	if language == "csharp" || language == "c#" || language == "cs" || language == "go" || language == "golang" {
		timeout = extendedToolchainTimeout
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	cmd, commandArgs := commandForPlatform(command, args)
	commandProcess := exec.CommandContext(ctx, cmd, commandArgs...)
	commandProcess.Dir = tempDir
	output, runErr := commandProcess.CombinedOutput()
	if runErr != nil && ctx.Err() == nil && (language == "javascript" || language == "js") {
		packageJSON := []byte("{\"type\":\"module\"}\n")
		if err := os.WriteFile(filepath.Join(tempDir, "package.json"), packageJSON, 0600); err != nil {
			return []Issue{toolchainFailureIssue("Unable to configure JavaScript module syntax validation: " + err.Error())}
		}
		commandProcess = exec.CommandContext(ctx, cmd, commandArgs...)
		commandProcess.Dir = tempDir
		output, runErr = commandProcess.CombinedOutput()
	}
	if ctx.Err() == context.DeadlineExceeded {
		return []Issue{toolchainFailureIssue(fmt.Sprintf("Syntax validation timed out after %s.", timeout))}
	}
	if runErr == nil {
		return nil
	}

	issues := parseToolDiagnostics(language, string(output), code)
	if len(issues) > 0 {
		return issues
	}
	return []Issue{toolchainFailureIssue(fmt.Sprintf("Syntax validation failed: %s", strings.TrimSpace(string(output))))}
}

func toolchainCommand(language, code string) (string, []string, string, string) {
	switch language {
	case "python", "py":
		python, ok := findExecutable("python", "python3", "py")
		if !ok {
			return "", nil, "", "Python parser unavailable: install Python and make 'python' available on PATH."
		}
		script := "import ast,sys; ast.parse(open(sys.argv[1], encoding='utf-8').read(), filename=sys.argv[1])"
		return python, []string{"-c", script, "{source}"}, "input.py", ""
	case "javascript", "js":
		node, ok := findExecutable("node")
		if !ok {
			return "", nil, "", "Node.js syntax checker unavailable: install Node.js and make 'node' available on PATH."
		}
		return node, []string{"--check", "{source}"}, "input.js", ""
	case "typescript", "ts":
		compiler, ok := findExecutable("tsc")
		if !ok {
			return "", nil, "", "TypeScript compiler unavailable: install TypeScript (for example, 'npm install -g typescript') and make 'tsc' available on PATH."
		}
		return compiler, []string{"--noEmit", "--pretty", "false", "--skipLibCheck", "--target", "ES2020", "--module", "commonjs", "{source}"}, "input.ts", ""
	case "go", "golang":
		compiler, ok := findExecutable("go")
		if !ok {
			return "", nil, "", "Go compiler unavailable: install Go and make 'go' available on PATH."
		}
		return compiler, []string{"build", "-buildmode=archive", "-o", filepath.Join("{temp}", "input.a"), "{source}"}, "input.go", ""
	case "c", "cpp", "c++":
		compilerName, mode := "g++", "c++"
		if language == "c" {
			compilerName, mode = "gcc", "c"
		}
		compiler, ok := findExecutable(compilerName)
		if !ok {
			return "", nil, "", fmt.Sprintf("%s compiler unavailable: install GCC and make '%s' available on PATH.", strings.ToUpper(compilerName), compilerName)
		}
		extension := ".cpp"
		if mode == "c" {
			extension = ".c"
		}
		return compiler, []string{"-fsyntax-only", "-x", mode, "{source}"}, "input" + extension, ""
	case "java":
		compiler, ok := findExecutable("javac")
		if !ok {
			return "", nil, "", "Java compiler unavailable: install a JDK and make 'javac' available on PATH."
		}
		filename := "Main.java"
		if matches := javaPublicType.FindStringSubmatch(code); len(matches) == 2 {
			filename = matches[1] + ".java"
		}
		return compiler, []string{"-proc:none", "-Xlint:none", "-d", "{temp}", "{source}"}, filename, ""
	case "csharp", "c#", "cs":
		dotnet, ok := findExecutable("dotnet")
		if !ok {
			return "", nil, "", ".NET compiler unavailable: install the .NET SDK and make 'dotnet' available on PATH."
		}
		output, err := exec.Command(dotnet, "--list-sdks").Output()
		if err != nil {
			return "", nil, "", ".NET SDK unavailable: 'dotnet --list-sdks' failed; install the .NET SDK."
		}
		versions := dotnetSDKVersion.FindAllStringSubmatch(string(output), -1)
		if len(versions) == 0 {
			return "", nil, "", ".NET SDK unavailable: install the .NET SDK; the .NET runtime alone cannot compile C#."
		}
		major := 0
		for _, version := range versions {
			if parsed, err := strconv.Atoi(version[1]); err == nil && parsed > major {
				major = parsed
			}
		}
		return dotnet, []string{"build", filepath.Join("{temp}", "CODETEST.csproj"), "--nologo", "--verbosity", "quiet", "-p:RestoreIgnoreFailedSources=true", "-p:TargetFramework=net" + strconv.Itoa(major) + ".0"}, "Program.cs", ""
	default:
		return "", nil, "", fmt.Sprintf("No external syntax validator is configured for %q.", language)
	}
}

func findExecutable(names ...string) (string, bool) {
	for _, name := range names {
		path, err := exec.LookPath(name)
		if err == nil {
			return path, true
		}
	}
	return "", false
}

func commandForPlatform(command string, args []string) (string, []string) {
	extension := strings.ToLower(filepath.Ext(command))
	if runtime.GOOS != "windows" || extension != ".cmd" && extension != ".bat" {
		return command, args
	}

	comspec := os.Getenv("COMSPEC")
	if comspec == "" {
		comspec = "cmd.exe"
	}
	script := `"` + strings.ReplaceAll(command, `"`, `""`) + `"`
	for _, arg := range args {
		script += ` "` + strings.ReplaceAll(arg, `"`, `""`) + `"`
	}
	return comspec, []string{"/d", "/s", "/c", script}
}

func parseToolDiagnostics(language, output, code string) []Issue {
	lines := strings.Split(strings.ReplaceAll(code, "\r\n", "\n"), "\n")
	var issues []Issue

	if language == "python" || language == "py" {
		line := 1
		message := ""
		for _, outputLine := range strings.Split(output, "\n") {
			if match := pythonFileLine.FindStringSubmatch(outputLine); len(match) == 2 {
				if parsed, err := strconv.Atoi(match[1]); err == nil {
					line = parsed
				}
			}
			if strings.Contains(outputLine, "SyntaxError:") ||
				strings.Contains(outputLine, "IndentationError:") ||
				strings.Contains(outputLine, "TabError:") {
				message = strings.TrimSpace(outputLine)
			}
		}
		if message != "" {
			issues = append(issues, diagnosticIssue(line, 0, "PYTHON_PARSE_ERROR", message, lines))
		}
		return issues
	}

	if language == "javascript" || language == "js" {
		outputLines := strings.Split(output, "\n")
		line := 1
		message := "JavaScript syntax error."
		for i, outputLine := range outputLines {
			if match := nodeFileLine.FindStringSubmatch(strings.TrimSpace(outputLine)); len(match) == 2 {
				if parsed, err := strconv.Atoi(match[1]); err == nil {
					line = parsed
				}
			}
			if strings.Contains(outputLine, "SyntaxError:") {
				message = strings.TrimSpace(outputLine)
				for previous := i - 1; previous >= 0 && i-previous <= 4; previous-- {
					if caret := strings.Index(outputLines[previous], "^"); caret >= 0 {
						return []Issue{diagnosticIssue(line, caret+1, "JAVASCRIPT_SYNTAX_ERROR", message, lines)}
					}
				}
			}
		}
		return []Issue{diagnosticIssue(line, 0, "JAVASCRIPT_SYNTAX_ERROR", message, lines)}
	}

	if language == "go" || language == "golang" {
		for _, outputLine := range strings.Split(output, "\n") {
			if match := goDiagnostic.FindStringSubmatch(outputLine); len(match) == 4 {
				issues = append(issues, diagnosticIssueFromMatch(match[1], match[2], "GO_COMPILER_ERROR", match[3], lines, "error"))
			}
		}
		return issues
	}

	for _, outputLine := range strings.Split(output, "\n") {
		var match []string
		rule := "COMPILER_SYNTAX_ERROR"
		switch language {
		case "c", "cpp", "c++":
			match = gccDiagnostic.FindStringSubmatch(outputLine)
			if len(match) == 5 {
				if match[3] == "warning" {
					issues = append(issues, diagnosticIssueFromMatch(match[1], match[2], "COMPILER_WARNING", match[4], lines, "warning"))
				} else if match[3] != "note" {
					issues = append(issues, diagnosticIssueFromMatch(match[1], match[2], rule, match[4], lines, "error"))
				}
			}
		case "java":
			match = javacDiagnostic.FindStringSubmatch(outputLine)
			if len(match) == 4 {
				if match[2] == "warning" {
					issues = append(issues, diagnosticIssueFromMatch(match[1], "", "JAVA_COMPILER_WARNING", match[3], lines, "warning"))
				} else {
					issues = append(issues, diagnosticIssueFromMatch(match[1], "", "JAVA_COMPILER_ERROR", match[3], lines, "error"))
				}
			}
		case "typescript", "ts":
			match = tscDiagnostic.FindStringSubmatch(outputLine)
			if len(match) == 5 {
				severity := "error"
				rule := "TYPESCRIPT_COMPILER_ERROR"
				if match[3] == "warning" {
					severity = "warning"
					rule = "TYPESCRIPT_COMPILER_WARNING"
				}
				issues = append(issues, diagnosticIssueFromMatch(match[1], match[2], rule, match[4], lines, severity))
			}
		case "csharp", "c#", "cs":
			match = dotnetDiagnostic.FindStringSubmatch(outputLine)
			if len(match) == 6 {
				severity := "error"
				rule := "CSHARP_COMPILER_ERROR"
				if match[3] == "warning" {
					severity = "warning"
					rule = "CSHARP_COMPILER_WARNING"
				}
				issues = append(issues, diagnosticIssueFromMatch(match[1], match[2], rule, match[5], lines, severity))
			}
		}
	}
	return issues
}

func diagnosticIssueFromMatch(lineText, columnText, rule, message string, sourceLines []string, severity string) Issue {
	line, _ := strconv.Atoi(lineText)
	column, _ := strconv.Atoi(columnText)
	issue := diagnosticIssue(line, column, rule, message, sourceLines)
	issue.Severity = severity
	if severity == "warning" {
		issue.Category = "Compiler Warning"
	}
	return issue
}

func diagnosticIssue(line, column int, rule, message string, sourceLines []string) Issue {
	if line < 1 {
		line = 1
	}
	snippet := ""
	if line <= len(sourceLines) {
		snippet = sourceLines[line-1]
	}
	return Issue{
		Line:       line,
		Column:     column,
		Severity:   "error",
		Category:   "Syntax Error",
		Rule:       rule,
		Message:    message,
		Snippet:    snippet,
		Suggestion: "Fix the parser or compiler diagnostic at the indicated location.",
	}
}

func toolchainUnavailableIssue(message string) Issue {
	return Issue{
		Line:       1,
		Severity:   "warning",
		Category:   "Parser / Compiler Unavailable",
		Rule:       "TOOLCHAIN_UNAVAILABLE",
		Message:    message,
		Snippet:    "",
		Suggestion: "Install the required language toolchain and restart CODETEST.",
	}
}

func toolchainFailureIssue(message string) Issue {
	return Issue{
		Line:       1,
		Severity:   "error",
		Category:   "Syntax Validation Failure",
		Rule:       "TOOLCHAIN_VALIDATION_FAILED",
		Message:    message,
		Suggestion: "Check that the configured parser or compiler is working, then try again.",
	}
}
