package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"

	"codetest/internal/analyzer"
)

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	mux := http.NewServeMux()

	// Static file serving
	fs := http.FileServer(http.Dir("./static"))
	mux.Handle("/", fs)

	// API endpoints
	mux.HandleFunc("/api/health", handleHealth)
	mux.HandleFunc("/api/analyze", handleAnalyze)
	mux.HandleFunc("/api/presets", handlePresets)

	addr := ":" + port
	fmt.Printf("\n=========================================\n")
	fmt.Printf(" [CODETEST] Server running on http://localhost:%s\n", port)
	fmt.Printf(" Software Tester & Code Review Engine\n")
	fmt.Printf("=========================================\n\n")

	if err := http.ListenAndServe(addr, enableCORS(mux)); err != nil {
		log.Fatalf("Server error: %v", err)
	}
}

func enableCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"status":  "healthy",
		"app":     "codetest",
		"version": "1.0.0",
	})
}

func handleAnalyze(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Bad Request: Unable to read body", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	var req analyzer.AnalyzeRequest
	if err := json.Unmarshal(body, &req); err != nil {
		http.Error(w, "Bad Request: Invalid JSON", http.StatusBadRequest)
		return
	}

	response := analyzer.Analyze(req)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func handlePresets(w http.ResponseWriter, r *http.Request) {
	presets := map[string]interface{}{
		"preset_20": map[string]interface{}{
			"title":       "20-Line Python Script (Multiple Syntax & Logic Bugs)",
			"language":    "python",
			"description": "Short script demonstrating missing colons, unclosed brackets, division by zero, and off-by-one errors.",
			"code": getPreset20(),
		},
		"preset_python": map[string]interface{}{
			"title":       "Python Parser and Logic Bug Test",
			"language":    "python",
			"description": "Python source with syntax and runtime-hazard examples for the Python parser and Python-specific checks.",
			"code": getPreset20(),
		},
		"preset_200": map[string]interface{}{
			"title":       "200-Line Web API Service (Python/JS Logic & Security Bugs)",
			"language":    "python",
			"description": "200-line server script with bracket mismatches, unclosed quotes, SQL injection hazards, and const reassignments.",
			"code": getPreset200(),
		},
		"preset_500": map[string]interface{}{
			"title":       "500-Line Enterprise Data Pipeline (Multi-Line Scaled Testing)",
			"language":    "python",
			"description": "500-line data processing pipeline showing line-by-line error identification across large file sizes.",
			"code": getPreset500(),
		},
		"preset_js": map[string]interface{}{
			"title":       "JavaScript / Node.js Buggy Module",
			"language":    "javascript",
			"description": "JavaScript code with const reassignments, NaN comparison, missing returns, and bracket issues.",
			"code": getPresetJS(),
		},
		"preset_go": map[string]interface{}{
			"title":       "Golang Concurrency & Syntax Bugs",
			"language":    "go",
			"description": "Go code with wg.Add() race conditions, unhandled errors, and AST syntax errors.",
			"code": getPresetGo(),
		},
		"preset_c": map[string]interface{}{
			"title":       "C Compiler Syntax Test",
			"language":    "c",
			"description": "C source with a missing statement terminator for GCC validation.",
			"code":        getPresetC(),
		},
		"preset_cpp": map[string]interface{}{
			"title":       "C++ Compiler Syntax Test",
			"language":    "cpp",
			"description": "C++ source with a missing statement terminator for G++ validation.",
			"code":        getPresetCpp(),
		},
		"preset_java": map[string]interface{}{
			"title":       "Java Compiler Syntax Test",
			"language":    "java",
			"description": "Java source with an invalid declaration for javac validation.",
			"code":        getPresetJava(),
		},
		"preset_ts": map[string]interface{}{
			"title":       "TypeScript Compiler Syntax Test",
			"language":    "typescript",
			"description": "TypeScript source with an invalid typed declaration for tsc validation.",
			"code":        getPresetTypeScript(),
		},
		"preset_null": map[string]interface{}{
			"title":       "Null / Empty Application (0-Line Test)",
			"language":    "generic",
			"description": "Tests that codetest analyzes and reports on an empty or null application.",
			"code":        "",
		},
		"preset_null_hazards": map[string]interface{}{
			"title":       "Null Pointer & NPE Hazards (Multi-Language)",
			"language":    "java",
			"description": "Java and C-style program demonstrating NullPointerException hazards, null dereferencing, and unboxing bugs.",
			"code":        getPresetNullHazards(),
		},
		"preset_rust": map[string]interface{}{
			"title":       "Rust Option None & Panic Hazards",
			"language":    "rust",
			"description": "Rust program with unwrap on None and missing semicolons.",
			"code":        getPresetRust(),
		},
		"preset_sql": map[string]interface{}{
			"title":       "SQL Syntax & Null Comparison Hazards",
			"language":    "sql",
			"description": "SQL script with WHERE = NULL traps and unconstrained UPDATE/DELETE statements.",
			"code":        getPresetSQL(),
		},
		"preset_csharp": map[string]interface{}{
			"title":       "C# Compiler Syntax Test",
			"language":    "csharp",
			"description": "C# source with an invalid declaration for .NET SDK validation.",
			"code":        getPresetCSharp(),
		},
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(presets)
}

func getPreset20() string {
	return `def calculate_average(items)
    total = 0
    # Line 4: Missing colon in if statement
    if len(items) == 0
        return 0
    
    # Line 8: Off-by-one loop boundary
    for i in range(len(items) + 1):
        total += items[i]
        
    # Line 12: Division by zero risk
    average = total / 0
    
    # Line 15: Unclosed bracket
    debug_info = [total, average
    print "Calculation complete!"
    
    # Line 19: Accidental assignment instead of comparison
    if (total = 100):
        print("Centurion total")
    return average`
}

func getPresetJS() string {
	return `// JavaScript / Node.js Sample with subtle bugs
function processUserTransactions(users) {
    const defaultTax = 0.15;

    // Bug: reading a property from a known null value
    const profile = null;
    console.log(profile.name);
    
    // Bug: const reassignment
    defaultTax = 0.20;
    
    for (let i = 0; i < users.length; i++) {
        let user = users[i];
        
        // Bug: NaN comparison is always false
        if (user.balance === NaN) {
            user.balance = 0;
        }
        
        // Bug: typeof comparison typo
        if (typeof user.age === 'int') {
            console.log("Valid age integer");
        }
        
        // Bug: Division by zero
        let factor = user.score / 0;
        
        // Bug: ASI hazard
        return
        {
            success: true
        };
        
        // Bug: Unreachable code
        console.log("Done user " + user.name);
    }
}`
}

func getPresetGo() string {
	return `package main

import (
	"fmt"
	"os"
	"sync"
)

func processFiles(filenames []string) {
	var wg sync.WaitGroup

	for _, fname := range filenames {
		// Bug: Calling wg.Add inside goroutine
		go func(f string) {
			wg.Add(1)
			defer wg.Done()
			
			// Bug: Ignored critical error
			file, _ := os.Open(f)
			defer file.Close()
			
			fmt.Println("Reading file:", f)
		}(fname)
	}

	// Bug: Division by zero
	ratio := 100 / 0
	fmt.Println("Ratio:", ratio)

	wg.Wait()
}

func main() {
	processFiles([]string{"data1.txt", "data2.txt"})
}`
}

func getPresetC() string {
	return `#include <stdio.h>

int main(void) {
    printf("C syntax test")
    return 0;
}`
}

func getPresetCpp() string {
	return `#include <iostream>

int main() {
    std::cout << "C++ syntax test" << std::endl
    return 0;
}`
}

func getPresetJava() string {
	return `public class SyntaxTest {
    public static void main(String[] args) {
        int count = ;
        System.out.println(count);
    }
}`
}

func getPresetTypeScript() string {
	return `interface User {
    name: string;
}

const user: User = { name: "Ada" };
const count: number = ;
console.log(user.name, count);`
}

func getPresetCSharp() string {
	return `using System;

public class SyntaxTest {
    public static void Main() {
        int count = ;
        Console.WriteLine(count);
    }
}`
}

func getPreset200() string {
	var b strings.Builder
	b.WriteString("# 200-Line Code Tester Benchmark Script\n")
	b.WriteString("# Simulating comprehensive production backend logic\n\n")

	for i := 1; i <= 200; i++ {
		switch i {
		case 15:
			b.WriteString("def authenticate_token(token)\n") // missing colon
		case 28:
			b.WriteString("    raw_query = f\"SELECT * FROM users WHERE id = \" + input_id\n") // SQL risk
		case 45:
			b.WriteString("    items_list = [1, 2, 3, 4, 5\n") // unclosed bracket
		case 72:
			b.WriteString("    calc_variance = total_val / 0\n") // division by zero
		case 105:
			b.WriteString("    if (status_code = 200):\n") // assignment in condition
		case 133:
			b.WriteString("        print \"Legacy python 2 print statement\"\n") // py2 print
		case 168:
			b.WriteString("    for idx in range(len(data_rows) + 1):\n") // off by one
		case 192:
			b.WriteString("        unclosed_string = 'missing quote string at end\n") // unclosed string
		default:
			b.WriteString(fmt.Sprintf("    # Line %d: Process step %d with valid operations\n", i, i))
			b.WriteString(fmt.Sprintf("    metric_%d = %d * 2\n", i, i))
			i++ // skip ahead to maintain line numbers
		}
	}
	return b.String()
}

func getPreset500() string {
	var b strings.Builder
	b.WriteString("# 500-Line Code Tester Enterprise Stress Test\n")
	b.WriteString("# Verifying accuracy across 500+ lines of real code\n\n")

	for i := 1; i <= 500; i++ {
		switch i {
		case 35:
			b.WriteString("def execute_worker_node(node_id)\n") // Missing colon
		case 90:
			b.WriteString("    db_query = \"SELECT name FROM accounts WHERE token = \" + req_token\n") // SQL Injection
		case 155:
			b.WriteString("    lookup_table = {\"key1\": 10, \"key2\": 20\n") // Unclosed bracket
		case 220:
			b.WriteString("    scale_rate = total_weight / 0\n") // Division by zero
		case 310:
			b.WriteString("    if (retry_count = 5):\n") // Assignment in condition
		case 385:
			b.WriteString("        error_log = \"invalid string literal\n") // Unclosed string
		case 440:
			b.WriteString("    for k in range(len(records) + 1):\n") // Off-by-one
		case 485:
			b.WriteString("        print \"Worker execution finished\"\n") // Py2 print
		default:
			b.WriteString(fmt.Sprintf("    step_%d_value = %d + 42\n", i, i))
		}
	}
	return b.String()
}

func getPresetNullHazards() string {
	return `public class NullSafetyDemo {
    public static void main(String[] args) {
        // Line 4: Null variable declaration
        String message = null;
        
        // Line 7: Calling method on null reference (throws NullPointerException)
        int len = message.length();
        
        // Line 10: Auto-unboxing null wrapper to primitive int (throws NPE)
        Integer score = null;
        int rawScore = score;
        
        // Line 14: Non-null-safe string comparison
        if (message.equals("Hello")) {
            System.out.println("Matched");
        }
    }
}`
}

func getPresetRust() string {
	return `fn main() {
    // Bug: Option initialized to None
    let name: Option<String> = None;
    
    // Bug: Calling .unwrap() on None causes panic at runtime
    let unwrap_val = name.unwrap();
    
    // Bug: Missing semicolon
    println!("Value: {}", unwrap_val)
}`
}

func getPresetSQL() string {
	return `-- SQL Null Comparison & Query Hazards
-- Bug 1: = NULL is always UNKNOWN in SQL (should use IS NULL)
SELECT user_id, email FROM users WHERE department_id = NULL;

-- Bug 2: UPDATE without WHERE clause modifies all records in the table
UPDATE accounts SET status = 'INACTIVE';

-- Bug 3: DELETE without WHERE clause deletes all rows
DELETE FROM audit_logs;`
}
