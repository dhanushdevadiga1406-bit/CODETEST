# 👾 CODETEST - Software Tester & Code Review Engine

**CODETEST** is a high-performance code review and software testing application that detects syntax mistakes, runtime crash hazards, and logical bugs across programs of any size—whether **20 lines, 200 lines, 500 lines, or more**.

Built with a **Go (Golang)** backend engine and an interactive **HTML + CSS + JavaScript** frontend featuring an authentic **8-bit retro pixel arcade aesthetic**.

---

## 🚀 Key Features

1. **Retro 8-Bit Pixel Experience**:
   - Animated pixel-art **CODETEST** title banner rendered in Google `'Press Start 2P'` typography.
   - Dynamic 8-bit canvas pixel avatar sprite with CRT scanlines and retro terminal badges.

2. **Precise Line-by-Line Error Identification**:
   - Prominent **Line Number Badges** (e.g. `[ LINE 14 ]`) displayed in high-contrast neon red and amber in the bottom panel.
   - **Click-to-Jump**: Clicking any error card or line badge immediately scrolls the editor directly to that exact line and highlights it.
   - Synchronized line gutter with real-time error icons (`✖` for errors, `▲` for warnings).

3. **Multi-Scale Program Support**:
   - Fast, accurate detection across **20 lines, 200 lines, 500 lines, or 1000+ lines**.
   - Built-in presets for instant testing:
     - `20 Lines Sample`: Missing colons, division by zero, unclosed brackets, off-by-one errors.
     - `200 Lines Sample`: Full simulated web application with bracket mismatches and SQL risks.
     - `500 Lines Benchmark`: Enterprise data pipeline stress testing scale and line resolution.
     - Language-specific test samples for Python, C, C++, Java, JavaScript, TypeScript, C#, and Go.

4. **Multi-Language Detection & Static Analysis**:
   - **Python**: Missing block colons, unclosed brackets/strings, Python 2 print syntax, mutable default arguments, `is` literal comparisons, bare `except:`.
   - **JavaScript / TypeScript**: Node.js syntax checks and TypeScript compiler checks, alongside unmatched brackets, reassignment to `const`, invalid `typeof`, `NaN` comparisons, unreachable statements after `return`, and ASI pitfalls.
   - **Go (Golang)**: Official Go AST parsing and compile-only Go compiler checks, plus goroutine race hazards and memory leaks (`time.Tick`).
   - **C / C++**: Missing semicolons, unsafe buffer operations (`gets`, `strcpy`), pointer double-free detection.
   - **Java**: Reference equality pitfalls with `String ==`, missing statement semicolons.
   - **C#**: .NET SDK compilation plus general syntax and logic checks.
   - **Universal / Generic**: Stack-based bracket balance, division by zero, assignments in conditional statements, infinite loop hazards.

---

## 🛠️ Architecture

```
codetest/
├── cmd/
│   └── (or root main.go)
├── internal/
│   └── analyzer/
│       ├── analyzer.go        # Orchestrator & language detector
│       ├── models.go          # Request, Response, Issue, and Summary structs
│       ├── python.go          # Python-specific syntax and linting checks
│       ├── javascript.go      # JS/TS syntax, ASI, and runtime bug rules
│       ├── golang.go          # Native Go AST parser & concurrency linters
│       ├── cpp.go             # C/C++ memory and syntax validators
│       ├── java.go            # Java syntax and logic rules
│       ├── generic.go         # Universal stack-based bracket/math checks
│       └── analyzer_test.go   # Automated unit and scale benchmarks
├── static/
│   ├── index.html             # Pixel-art UI and responsive workspace
│   ├── style.css              # Cyberpunk/Retro 8-bit styling
│   └── app.js                 # Line sync, canvas rendering, and API caller
├── go.mod                     # Go module definition
├── codetest.exe               # Compiled standalone server binary
└── README.md                  # Documentation
```

---

## 🏃 Running CODETEST

### Option 1: Run the Precompiled Binary
```powershell
.\codetest.exe
```

### Option 2: Run with Go
```powershell
go run main.go
```

Open your browser to:
```
http://localhost:8080
```

Choose a language-specific test from **PRESETS** to see that language's parser/compiler checks in action. CODETEST starts with an empty editor so it doesn't assume every source file is Python.

### Parser and compiler checks

In addition to its existing static checks, CODETEST uses language-specific parsers and compilers without running uploaded programs:

- Python uses the Python interpreter's `ast` parser.
- C and C++ use `gcc` and `g++` in syntax-only mode.
- Java uses `javac` with annotation processing disabled.
- JavaScript uses `node --check`; TypeScript uses `tsc --noEmit`.
- C# builds a temporary project with the installed .NET SDK.
- Go uses the Go compiler in archive-build mode.

Install the corresponding toolchain and make its executable available on `PATH` to enable that language's syntax validation. If it is missing, the analyzer reports that explicitly. All compiler checks are syntax/type checks only; uploaded programs are never executed.

---

## 📡 REST API Reference

### 1. `POST /api/analyze`
Analyze source code.

**Request:**
```json
{
  "code": "def divide(a, b):\n    return a / 0",
  "language": "python"
}
```

**Response:**
```json
{
  "success": true,
  "language": "python",
  "total_lines": 2,
  "issues": [
    {
      "line": 2,
      "severity": "error",
      "category": "Runtime Crash Hazard",
      "rule": "DIVISION_BY_ZERO",
      "message": "Division or modulo by zero constant detected.",
      "snippet": "    return a / 0",
      "suggestion": "Ensure the divisor is non-zero or add a validation check prior to division."
    }
  ],
  "summary": {
    "total_errors": 1,
    "total_warnings": 0,
    "total_infos": 0,
    "execution_time_ms": 0.45,
    "status": "ERRORS_FOUND"
  }
}
```

### 2. `GET /api/presets`
Fetches preloaded buggy test programs (20, 200, 500 lines).

### 3. `GET /api/health`
Health check endpoint returning server status.
