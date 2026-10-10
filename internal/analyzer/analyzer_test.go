package analyzer

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"
)

func TestPythonBugDetection(t *testing.T) {
	code := `def calculate(x)
    total = x / 0
    items = [1, 2, 3
    if (total = 5):
        print "result"`

	res := Analyze(AnalyzeRequest{
		Code:     code,
		Language: "python",
	})

	if !res.Success {
		t.Fatalf("Expected success to be true")
	}

	if len(res.Issues) == 0 {
		t.Fatalf("Expected errors to be detected, got 0")
	}

	linesWithIssues := make(map[int]bool)
	for _, issue := range res.Issues {
		linesWithIssues[issue.Line] = true
	}

	if !linesWithIssues[1] {
		t.Errorf("Expected issue at Line 1 for missing colon")
	}
	if !linesWithIssues[2] {
		t.Errorf("Expected issue at Line 2 for division by zero")
	}
	if !linesWithIssues[3] {
		t.Errorf("Expected issue at Line 3 for unclosed bracket")
	}
	if !linesWithIssues[5] {
		t.Errorf("Expected issue at Line 5 for python 2 print")
	}
}

func TestCleanSourceReportsNoErrors(t *testing.T) {
	result := Analyze(AnalyzeRequest{
		Code:     "value = 1 + 2",
		Language: "generic",
	})
	if !result.Success {
		t.Fatal("Expected clean source analysis to succeed")
	}
	if result.Summary.Status != "PASSED" {
		t.Errorf("Expected PASSED status, got %q", result.Summary.Status)
	}
	if result.Summary.TotalErrors != 0 || result.Summary.TotalWarnings != 0 {
		t.Errorf("Expected no errors or warnings, got %+v", result.Summary)
	}
	if result.Issues == nil {
		t.Fatal("Expected a non-nil empty issue list for clean source")
	}
	encoded, err := json.Marshal(result)
	if err != nil {
		t.Fatalf("Could not encode clean analysis response: %v", err)
	}
	if strings.Contains(string(encoded), `"issues":null`) {
		t.Errorf("Clean analysis serialized issues as null: %s", encoded)
	}
}

func TestScale500Lines(t *testing.T) {
	var b strings.Builder
	for i := 1; i <= 500; i++ {
		if i == 250 {
			b.WriteString("val = total / 0\n")
		} else if i == 400 {
			b.WriteString("arr = [1, 2, 3\n")
		} else {
			b.WriteString(fmt.Sprintf("x_%d = %d + 1\n", i, i))
		}
	}

	res := Analyze(AnalyzeRequest{
		Code:     b.String(),
		Language: "python",
	})

	if res.TotalLines != 500 {
		t.Errorf("Expected 500 lines, got %d", res.TotalLines)
	}

	found250 := false
	found400 := false
	for _, issue := range res.Issues {
		if issue.Line == 250 && issue.Rule == "DIVISION_BY_ZERO" {
			found250 = true
		}
		if issue.Line == 400 && issue.Rule == "UNCLOSED_OPEN_BRACKET" {
			found400 = true
		}
	}

	if !found250 {
		t.Errorf("Failed to identify division by zero at Line 250 in 500-line file")
	}
	if !found400 {
		t.Errorf("Failed to identify unclosed bracket at Line 400 in 500-line file")
	}
}

func TestJavaScriptConstReassignment(t *testing.T) {
	code := `const maxCount = 100;
maxCount = 200;
if (val === NaN) {
    console.log("Bug");
}`

	res := Analyze(AnalyzeRequest{
		Code:     code,
		Language: "javascript",
	})

	foundReassign := false
	foundNan := false
	for _, issue := range res.Issues {
		if issue.Line == 2 && issue.Rule == "CONST_REASSIGNMENT" {
			foundReassign = true
		}
		if issue.Line == 3 && issue.Rule == "NAN_COMPARISON" {
			foundNan = true
		}
	}

	if !foundReassign {
		t.Errorf("Expected const reassignment error on Line 2")
	}
	if !foundNan {
		t.Errorf("Expected NaN comparison error on Line 3")
	}
}

func TestJavaScriptNullPropertyAccess(t *testing.T) {
	tests := []struct {
		name     string
		code     string
		wantRule bool
	}{
		{
			name: "property read after null assignment",
			code: "const user = null;\nconsole.log(user.name);",
			wantRule: true,
		},
		{
			name: "optional chaining",
			code: "const user = null;\nconsole.log(user?.name);",
			wantRule: false,
		},
		{
			name: "reassigned to object before property read",
			code: "let user = null;\nuser = {};\nconsole.log(user.name);",
			wantRule: false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result := Analyze(AnalyzeRequest{Code: test.code, Language: "javascript"})
			found := false
			for _, issue := range result.Issues {
				if issue.Rule == "NULL_PROPERTY_ACCESS" {
					found = true
					if issue.Line != 2 && test.name == "property read after null assignment" {
						t.Errorf("Expected null property access on line 2, got line %d", issue.Line)
					}
				}
			}
			if found != test.wantRule {
				t.Errorf("NULL_PROPERTY_ACCESS found = %t, want %t; issues: %+v", found, test.wantRule, result.Issues)
			}
		})
	}
}

func TestGolangASTAnalysis(t *testing.T) {
	code := `package main

import "fmt"

func main() {
    x := 10 / 0
    fmt.Println(x
}`

	res := Analyze(AnalyzeRequest{
		Code:     code,
		Language: "go",
	})

	if len(res.Issues) == 0 {
		t.Fatalf("Expected Go issues to be detected")
	}

	hasSyntaxError := false
	for _, issue := range res.Issues {
		if issue.Rule == "GO_SYNTAX_ERROR" || issue.Rule == "UNCLOSED_OPEN_BRACKET" {
			hasSyntaxError = true
		}
	}

	if !hasSyntaxError {
		t.Errorf("Expected syntax error to be caught in Go code")
	}
}

func TestCppDoubleFree(t *testing.T) {
	code := `#include <iostream>
int main() {
    char* buf = (char*)malloc(10);
    free(buf);
    free(buf);
    return 0;
}`

	res := Analyze(AnalyzeRequest{
		Code:     code,
		Language: "cpp",
	})

	foundDoubleFree := false
	for _, issue := range res.Issues {
		if issue.Line == 5 && issue.Rule == "CPP_DOUBLE_FREE" {
			foundDoubleFree = true
		}
	}

	if !foundDoubleFree {
		t.Errorf("Expected double free detected on Line 5")
	}
}

func TestPythonParserDiagnostic(t *testing.T) {
	issues := parseToolDiagnostics("python", `  File "input.py", line 2
    if True
           ^
SyntaxError: expected ':'`, "if True\n    pass")

	if len(issues) != 1 {
		t.Fatalf("Expected one Python parser diagnostic, got %d", len(issues))
	}
	if issues[0].Line != 2 || issues[0].Rule != "PYTHON_PARSE_ERROR" {
		t.Errorf("Unexpected Python diagnostic: %+v", issues[0])
	}
}

func TestJavaScriptParserDiagnostic(t *testing.T) {
	output := `C:\temp\input.js:2
const value = ;
              ^

SyntaxError: Unexpected token ';'`
	issues := parseToolDiagnostics("javascript", output, "const valid = true;\nconst value = ;")

	if len(issues) != 1 {
		t.Fatalf("Expected one JavaScript parser diagnostic, got %d", len(issues))
	}
	if issues[0].Line != 2 || issues[0].Column == 0 || issues[0].Rule != "JAVASCRIPT_SYNTAX_ERROR" {
		t.Errorf("Unexpected JavaScript diagnostic: %+v", issues[0])
	}
}

func TestJavaScriptModuleSyntaxValidation(t *testing.T) {
	if _, ok := findExecutable("node"); !ok {
		t.Skip("Node.js is not installed")
	}

	result := Analyze(AnalyzeRequest{
		Code:     "import value from './module.js';\nexport const answer = value;",
		Language: "javascript",
	})
	for _, issue := range result.Issues {
		if issue.Rule == "JAVASCRIPT_SYNTAX_ERROR" {
			t.Errorf("Valid JavaScript module was rejected: %+v", issue)
		}
	}
}

func TestCompilerDiagnostics(t *testing.T) {
	tests := []struct {
		name     string
		language string
		output   string
		rule     string
	}{
		{
			name:     "C++ compiler",
			language: "cpp",
			output:   "input.cpp:2:8: error: expected ';' before '}' token",
			rule:     "COMPILER_SYNTAX_ERROR",
		},
		{
			name:     "Java compiler",
			language: "java",
			output:   "Main.java:2: error: ';' expected",
			rule:     "JAVA_COMPILER_ERROR",
		},
		{
			name:     "TypeScript compiler",
			language: "typescript",
			output:   "C:\\temp\\input.ts(2,8): error TS1005: ';' expected.",
			rule:     "TYPESCRIPT_COMPILER_ERROR",
		},
		{
			name:     "C# compiler",
			language: "csharp",
			output:   "C:\\temp\\Program.cs(2,8): error CS1002: ; expected",
			rule:     "CSHARP_COMPILER_ERROR",
		},
		{
			name:     "Go compiler",
			language: "go",
			output:   "# command-line-arguments\n./input.go:2:8: syntax error: unexpected }, expected expression",
			rule:     "GO_COMPILER_ERROR",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			issues := parseToolDiagnostics(test.language, test.output, "class Main {\n}")
			if len(issues) != 1 {
				t.Fatalf("Expected one compiler diagnostic, got %d", len(issues))
			}
			if issues[0].Line != 2 || issues[0].Rule != test.rule {
				t.Errorf("Unexpected compiler diagnostic: %+v", issues[0])
			}
		})
	}
}

func TestDetectCLanguage(t *testing.T) {
	if got := DetectLanguage("#include <stdio.h>\nint main(void) { return 0; }"); got != "c" {
		t.Errorf("Expected C detection, got %q", got)
	}
	if got := DetectLanguage("#include <iostream>\nint main() { std::cout << 1; }"); got != "cpp" {
		t.Errorf("Expected C++ detection, got %q", got)
	}
}

func TestDetectLanguageForImports(t *testing.T) {
	tests := []struct {
		name string
		code string
		want string
	}{
		{
			name: "JavaScript module with object literal",
			code: "import client from './client.js';\nconst config = { timeout: 30 };",
			want: "javascript",
		},
		{
			name: "Python import-only source",
			code: "import os\nfrom pathlib import Path",
			want: "python",
		},
		{
			name: "TypeScript interface",
			code: "interface User { name: string; }",
			want: "typescript",
		},
		{
			name: "Java interface",
			code: "public interface User { void save(); }",
			want: "java",
		},
		{
			name: "C# source",
			code: "using System;\npublic class App { static void Main() { Console.WriteLine(\"hi\"); } }",
			want: "csharp",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := DetectLanguage(test.code); got != test.want {
				t.Errorf("DetectLanguage() = %q, want %q", got, test.want)
			}
		})
	}
}

func TestAnalyzeRunsAvailableSyntaxValidators(t *testing.T) {
	tests := []struct {
		language string
		code     string
		rule     string
		tool     string
	}{
		{
			language: "python",
			code:     "def broken(:\n    pass",
			rule:     "PYTHON_PARSE_ERROR",
			tool:     "python",
		},
		{
			language: "c",
			code:     "int main(void) { return 0 }",
			rule:     "COMPILER_SYNTAX_ERROR",
			tool:     "gcc",
		},
		{
			language: "javascript",
			code:     "const value = ;",
			rule:     "JAVASCRIPT_SYNTAX_ERROR",
			tool:     "node",
		},
		{
			language: "typescript",
			code:     "const value: number = ;",
			rule:     "TYPESCRIPT_COMPILER_ERROR",
			tool:     "tsc",
		},
		{
			language: "java",
			code:     "public class Probe { void run() { int value = ; } }",
			rule:     "JAVA_COMPILER_ERROR",
			tool:     "javac",
		},
		{
			language: "csharp",
			code:     "public class Probe { public void Run() { int value = ; } }",
			rule:     "CSHARP_COMPILER_ERROR",
			tool:     "dotnet",
		},
	}
	for _, test := range tests {
		t.Run(test.language, func(t *testing.T) {
			command, _, _, unavailable := toolchainCommand(test.language, test.code)
			if command == "" {
				result := Analyze(AnalyzeRequest{Code: test.code, Language: test.language})
				for _, issue := range result.Issues {
					if issue.Rule == "TOOLCHAIN_UNAVAILABLE" && issue.Severity == "warning" {
						return
					}
				}
				t.Errorf("Expected unavailable %s to be reported (%s); got %+v", test.tool, unavailable, result.Issues)
				return
			}
			result := Analyze(AnalyzeRequest{Code: test.code, Language: test.language})
			for _, issue := range result.Issues {
				if issue.Rule == test.rule {
					return
				}
			}
			t.Errorf("Expected %s syntax validator to report %s; got %+v", test.tool, test.rule, result.Issues)
		})
	}
}

func TestNullApplicationDetection(t *testing.T) {
	// 1. Completely empty code
	resEmpty := Analyze(AnalyzeRequest{
		Code:     "",
		Language: "auto",
	})
	if len(resEmpty.Issues) == 0 || resEmpty.Issues[0].Rule != "NULL_OR_EMPTY_APPLICATION" {
		t.Errorf("Expected NULL_OR_EMPTY_APPLICATION on empty code, got %+v", resEmpty.Issues)
	}
	if resEmpty.Issues[0].Line != 1 {
		t.Errorf("Expected Line 1 for null application error, got %d", resEmpty.Issues[0].Line)
	}

	// 2. Whitespace-only code
	resWhitespace := Analyze(AnalyzeRequest{
		Code:     "   \n\t  \n  ",
		Language: "python",
	})
	if len(resWhitespace.Issues) == 0 || resWhitespace.Issues[0].Rule != "NULL_OR_EMPTY_APPLICATION" {
		t.Errorf("Expected NULL_OR_EMPTY_APPLICATION on whitespace code, got %+v", resWhitespace.Issues)
	}
}

func TestMultiLanguageNullHazardDetection(t *testing.T) {
	// Python None Dereference
	pyRes := Analyze(AnalyzeRequest{
		Code:     "data = None\nprint(data.name)",
		Language: "python",
	})
	foundPyNull := false
	for _, issue := range pyRes.Issues {
		if issue.Rule == "PYTHON_NULL_VARIABLE_DEREFERENCE" && issue.Line == 2 {
			foundPyNull = true
		}
	}
	if !foundPyNull {
		t.Errorf("Expected PYTHON_NULL_VARIABLE_DEREFERENCE on Line 2 in Python, got %+v", pyRes.Issues)
	}

	// Java NullPointerException
	javaRes := Analyze(AnalyzeRequest{
		Code:     "String s = null;\nint len = s.length();",
		Language: "java",
	})
	foundJavaNpe := false
	for _, issue := range javaRes.Issues {
		if issue.Rule == "JAVA_NULL_POINTER_EXCEPTION" && issue.Line == 2 {
			foundJavaNpe = true
		}
	}
	if !foundJavaNpe {
		t.Errorf("Expected JAVA_NULL_POINTER_EXCEPTION on Line 2 in Java, got %+v", javaRes.Issues)
	}

	// C NULL Pointer Dereference
	cRes := Analyze(AnalyzeRequest{
		Code:     "int* ptr = NULL;\n*ptr = 42;",
		Language: "c",
	})
	foundCNull := false
	for _, issue := range cRes.Issues {
		if issue.Rule == "C_NULL_POINTER_DEREFERENCE" && issue.Line == 2 {
			foundCNull = true
		}
	}
	if !foundCNull {
		t.Errorf("Expected C_NULL_POINTER_DEREFERENCE on Line 2 in C, got %+v", cRes.Issues)
	}

	// Go Nil Map Write
	goRes := Analyze(AnalyzeRequest{
		Code:     "package main\nvar counts map[string]int\nfunc init() {\ncounts[\"test\"] = 1\n}",
		Language: "go",
	})
	foundGoNilMap := false
	for _, issue := range goRes.Issues {
		if issue.Rule == "GO_NIL_MAP_WRITE" && issue.Line == 4 {
			foundGoNilMap = true
		}
	}
	if !foundGoNilMap {
		t.Errorf("Expected GO_NIL_MAP_WRITE on Line 4 in Go, got %+v", goRes.Issues)
	}

	// Rust Option None unwrap
	rustRes := Analyze(AnalyzeRequest{
		Code:     "fn main() {\nlet opt: Option<i32> = None;\nlet val = opt.unwrap();\n}",
		Language: "rust",
	})
	foundRustNone := false
	for _, issue := range rustRes.Issues {
		if issue.Rule == "RUST_UNWRAP_ON_NONE" && issue.Line == 3 {
			foundRustNone = true
		}
	}
	if !foundRustNone {
		t.Errorf("Expected RUST_UNWRAP_ON_NONE on Line 3 in Rust, got %+v", rustRes.Issues)
	}

	// SQL = NULL trap
	sqlRes := Analyze(AnalyzeRequest{
		Code:     "SELECT * FROM users WHERE status = NULL;",
		Language: "sql",
	})
	foundSqlNull := false
	for _, issue := range sqlRes.Issues {
		if issue.Rule == "SQL_EQUALS_NULL_TRAP" && issue.Line == 1 {
			foundSqlNull = true
		}
	}
	if !foundSqlNull {
		t.Errorf("Expected SQL_EQUALS_NULL_TRAP on Line 1 in SQL, got %+v", sqlRes.Issues)
	}
}

func TestCNullPointerHeuristicDoesNotRequireCompilerHeader(t *testing.T) {
	lines := []string{"int* ptr = NULL;", "*ptr = 42;"}
	issues := AnalyzeC(lines)
	for _, issue := range issues {
		if issue.Rule == "C_NULL_POINTER_DEREFERENCE" && issue.Line == 2 {
			return
		}
	}
	t.Fatalf("Expected C null dereference heuristic to flag line 2, got %+v", issues)
}

func TestNoPythonRulesLeakToOtherLanguages(t *testing.T) {
	// C code with preprocessor #include should NOT be treated as Python comment or complain about Python colons
	cCode := "#include <stdio.h>\nint main(void) {\n    printf(\"Hello\\n\");\n    return 0;\n}"
	res := Analyze(AnalyzeRequest{
		Code:     cCode,
		Language: "c",
	})
	for _, issue := range res.Issues {
		if strings.Contains(issue.Rule, "PYTHON") {
			t.Errorf("Python rule leaked into C analysis: %+v", issue)
		}
	}

	// Java code should NOT trigger Python missing colons on if statements
	javaCode := "public class Test {\n    public static void main(String[] args) {\n        if (args.length > 0) {\n            System.out.println(args[0]);\n        }\n    }\n}"
	resJava := Analyze(AnalyzeRequest{
		Code:     javaCode,
		Language: "java",
	})
	for _, issue := range resJava.Issues {
		if strings.Contains(issue.Rule, "PYTHON") {
			t.Errorf("Python rule leaked into Java analysis: %+v", issue)
		}
	}
}
