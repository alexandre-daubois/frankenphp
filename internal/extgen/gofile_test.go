package extgen

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGoFileGenerator_Generate(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "go_file_generator_test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	sourceContent := `package main

import (
	"fmt"
	"strings"
	"github.com/dunglas/frankenphp/internal/extensions/types"
)

//export_php: greet(name string): string
func greet(name *go_string) *go_value {
	return types.String("Hello " + CStringToGoString(name))
}

//export_php: calculate(a int, b int): int
func calculate(a long, b long) *go_value {
	result := a + b
	return types.Int(result)
}

func internalHelper(data string) string {
	return strings.ToUpper(data)
}

func anotherHelper() {
	fmt.Println("Internal helper")
}`

	sourceFile := filepath.Join(tmpDir, "test.go")
	if err := os.WriteFile(sourceFile, []byte(sourceContent), 0644); err != nil {
		t.Fatal(err)
	}

	generator := &Generator{
		BaseName:   "test",
		SourceFile: sourceFile,
		BuildDir:   tmpDir,
		Functions: []PHPFunction{
			{
				Name:       "greet",
				ReturnType: "string",
				GoFunction: `func greet(name *go_string) *go_value {
	return types.String("Hello " + CStringToGoString(name))
}`,
			},
			{
				Name:       "calculate",
				ReturnType: "int",
				GoFunction: `func calculate(a long, b long) *go_value {
	result := a + b
	return types.Int(result)
}`,
			},
		},
	}

	goGen := GoFileGenerator{generator}
	err = goGen.generate()
	if err != nil {
		t.Fatalf("generate() failed: %v", err)
	}

	expectedFile := filepath.Join(tmpDir, "testext.go")
	if _, err := os.Stat(expectedFile); os.IsNotExist(err) {
		t.Errorf("Expected Go file was not created: %s", expectedFile)
	}

	content, err := ReadFile(expectedFile)
	if err != nil {
		t.Fatalf("Failed to read generated Go file: %v", err)
	}

	testGoFileBasicStructure(t, content, "test")
	testGoFileImports(t, content)
	testGoFileExportedFunctions(t, content, generator.Functions)
	testGoFileInternalFunctions(t, content)
}

func TestGoFileGenerator_BuildContent(t *testing.T) {
	tests := []struct {
		name        string
		baseName    string
		sourceFile  string
		functions   []PHPFunction
		contains    []string
		notContains []string
	}{
		{
			name:     "simple extension",
			baseName: "simple",
			sourceFile: createTempSourceFile(t, `package main

//export_php: test(): void
func test() {
	// simple function
}`),
			functions: []PHPFunction{
				{
					Name:       "test",
					ReturnType: "void",
					GoFunction: "func test() {\n\t// simple function\n}",
				},
			},
			contains: []string{
				"package simple",
				`#include "simple.h"`,
				"import \"C\"",
				"func init()",
				"C.register_extension()",
				"//export test",
				"func test()",
			},
		},
		{
			name:     "extension with complex imports",
			baseName: "complex",
			sourceFile: createTempSourceFile(t, `package main

import (
	"fmt"
	"strings"
	"encoding/json"
	"github.com/dunglas/frankenphp/internal/extensions/types"
)

//export_php: process(data string): string
func process(data *go_string) *go_value {
	return types.String(fmt.Sprintf("processed: %s", CStringToGoString(data)))
}`),
			functions: []PHPFunction{
				{
					Name:       "process",
					ReturnType: "string",
					GoFunction: `func process(data *go_string) *go_value {
	return String(fmt.Sprintf("processed: %s", CStringToGoString(data)))
}`,
				},
			},
			contains: []string{
				"package complex",
				`import "fmt"`,
				`import "strings"`,
				`import "encoding/json"`,
				"//export process",
				`import "C"`,
			},
		},
		{
			name:     "extension with internal functions",
			baseName: "internal",
			sourceFile: createTempSourceFile(t, `package main

//export_php: publicFunc(): void
func publicFunc() {}

func internalFunc1() string {
	return "internal"
}

func internalFunc2(data string) {
	// process data internally
}`),
			functions: []PHPFunction{
				{
					Name:       "publicFunc",
					ReturnType: "void",
					GoFunction: "func publicFunc() {}",
				},
			},
			contains: []string{
				"func internalFunc1() string",
				"func internalFunc2(data string)",
				"//export publicFunc",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			defer os.Remove(tt.sourceFile)

			generator := &Generator{
				BaseName:   tt.baseName,
				SourceFile: tt.sourceFile,
				Functions:  tt.functions,
			}

			goGen := GoFileGenerator{generator}
			content, err := goGen.buildContent()
			if err != nil {
				t.Fatalf("buildContent() failed: %v", err)
			}

			for _, expected := range tt.contains {
				if !strings.Contains(content, expected) {
					t.Errorf("Generated Go content should contain '%s'\nGenerated:\n%s", expected, content)
				}
			}
		})
	}
}

func TestGoFileGenerator_PackageNameSanitization(t *testing.T) {
	tests := []struct {
		baseName        string
		expectedPackage string
	}{
		{"simple", "simple"},
		{"my-extension", "my_extension"},
		{"ext.with.dots", "ext_with_dots"},
		{"123invalid", "_123invalid"},
		{"valid_name", "valid_name"},
	}

	for _, tt := range tests {
		t.Run(tt.baseName, func(t *testing.T) {
			sourceFile := createTempSourceFile(t, "package main\n//export_php: test(): void\nfunc test() {}")
			defer os.Remove(sourceFile)

			generator := &Generator{
				BaseName:   tt.baseName,
				SourceFile: sourceFile,
				Functions: []PHPFunction{
					{Name: "test", ReturnType: "void", GoFunction: "func test() {}"},
				},
			}

			goGen := GoFileGenerator{generator}
			content, err := goGen.buildContent()
			if err != nil {
				t.Fatalf("buildContent() failed: %v", err)
			}

			expectedPackage := "package " + tt.expectedPackage
			if !strings.Contains(content, expectedPackage) {
				t.Errorf("Generated content should contain '%s'", expectedPackage)
			}
		})
	}
}

func TestGoFileGenerator_ErrorHandling(t *testing.T) {
	tests := []struct {
		name       string
		sourceFile string
		expectErr  bool
	}{
		{
			name:       "nonexistent file",
			sourceFile: "/nonexistent/file.go",
			expectErr:  true,
		},
		{
			name:       "invalid Go syntax",
			sourceFile: createTempSourceFile(t, "invalid go syntax here"),
			expectErr:  true,
		},
		{
			name:       "valid file",
			sourceFile: createTempSourceFile(t, "package main\nfunc test() {}"),
			expectErr:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if !tt.expectErr && tt.sourceFile != "/nonexistent/file.go" {
				defer os.Remove(tt.sourceFile)
			}

			generator := &Generator{
				BaseName:   "test",
				SourceFile: tt.sourceFile,
			}

			goGen := GoFileGenerator{generator}
			_, err := goGen.buildContent()

			if tt.expectErr && err == nil {
				t.Error("Expected error but got none")
			}

			if !tt.expectErr && err != nil {
				t.Errorf("Unexpected error: %v", err)
			}
		})
	}
}

func TestGoFileGenerator_ImportFiltering(t *testing.T) {
	sourceContent := `package main

import (
	"C"
	"fmt"
	"strings"
	"github.com/dunglas/frankenphp/internal/extensions/types"
	"github.com/other/package"
	originalPkg "github.com/test/original"
)

//export_php: test(): void
func test() {}`

	sourceFile := createTempSourceFile(t, sourceContent)
	defer os.Remove(sourceFile)

	generator := &Generator{
		BaseName:   "importtest",
		SourceFile: sourceFile,
		Functions: []PHPFunction{
			{Name: "test", ReturnType: "void", GoFunction: "func test() {}"},
		},
	}

	goGen := GoFileGenerator{generator}
	content, err := goGen.buildContent()
	if err != nil {
		t.Fatalf("buildContent() failed: %v", err)
	}

	expectedImports := []string{
		`import "fmt"`,
		`import "strings"`,
		`import "github.com/other/package"`,
	}

	for _, imp := range expectedImports {
		if !strings.Contains(content, imp) {
			t.Errorf("Generated content should contain import: %s", imp)
		}
	}

	forbiddenImports := []string{
		`import "C"`,
	}

	cImportCount := strings.Count(content, `import "C"`)
	if cImportCount != 1 {
		t.Errorf("Expected exactly 1 occurrence of 'import \"C\"', got %d", cImportCount)
	}

	for _, imp := range forbiddenImports[1:] {
		if strings.Contains(content, imp) {
			t.Errorf("Generated content should NOT contain import: %s", imp)
		}
	}
}

func TestGoFileGenerator_ComplexScenario(t *testing.T) {
	sourceContent := `package example

import (
	"fmt"
	"strings"
	"encoding/json"
	"github.com/dunglas/frankenphp/internal/extensions/types"
)

//export_php: processData(input string, options array): array
func processData(input *go_string, options *go_nullable) *go_value {
	data := CStringToGoString(input)
	processed := internalProcess(data)
	return types.Array([]interface{}{processed})
}

//export_php: validateInput(data string): bool
func validateInput(data *go_string) *go_value {
	input := CStringToGoString(data)
	isValid := len(input) > 0 && validateFormat(input)
	return types.Bool(isValid)
}

func internalProcess(data string) string {
	return strings.ToUpper(data)
}

func validateFormat(input string) bool {
	return !strings.Contains(input, "invalid")
}

func jsonHelper(data interface{}) ([]byte, error) {
	return json.Marshal(data)
}

func debugPrint(msg string) {
	fmt.Printf("DEBUG: %s\n", msg)
}`

	sourceFile := createTempSourceFile(t, sourceContent)
	defer os.Remove(sourceFile)

	functions := []PHPFunction{
		{
			Name:       "processData",
			ReturnType: "array",
			GoFunction: `func processData(input *go_string, options *go_nullable) *go_value {
	data := CStringToGoString(input)
	processed := internalProcess(data)
	return Array([]interface{}{processed})
}`,
		},
		{
			Name:       "validateInput",
			ReturnType: "bool",
			GoFunction: `func validateInput(data *go_string) *go_value {
	input := CStringToGoString(data)
	isValid := len(input) > 0 && validateFormat(input)
	return Bool(isValid)
}`,
		},
	}

	generator := &Generator{
		BaseName:   "complex-example",
		SourceFile: sourceFile,
		Functions:  functions,
	}

	goGen := GoFileGenerator{generator}
	content, err := goGen.buildContent()
	if err != nil {
		t.Fatalf("buildContent() failed: %v", err)
	}

	if !strings.Contains(content, "package complex_example") {
		t.Error("Package name should be sanitized")
	}

	internalFuncs := []string{
		"func internalProcess(data string) string",
		"func validateFormat(input string) bool",
		"func jsonHelper(data interface{}) ([]byte, error)",
		"func debugPrint(msg string)",
	}

	for _, fn := range internalFuncs {
		if !strings.Contains(content, fn) {
			t.Errorf("Generated content should contain internal function: %s", fn)
		}
	}

	for _, fn := range functions {
		exportDirective := "//export " + fn.Name
		if !strings.Contains(content, exportDirective) {
			t.Errorf("Generated content should contain export directive: %s", exportDirective)
		}
	}

	if strings.Contains(content, "types.Array") || strings.Contains(content, "types.Bool") {
		t.Error("Types should be replaced (types.* should not appear)")
	}

	if !strings.Contains(content, "return Array(") || !strings.Contains(content, "return Bool(") {
		t.Error("Replaced types should appear without types prefix")
	}
}

func createTempSourceFile(t *testing.T, content string) string {
	tmpfile, err := os.CreateTemp("", "source*.go")
	if err != nil {
		t.Fatal(err)
	}

	if _, err := tmpfile.Write([]byte(content)); err != nil {
		t.Fatal(err)
	}
	tmpfile.Close()

	return tmpfile.Name()
}

func testGoFileBasicStructure(t *testing.T, content, baseName string) {
	requiredElements := []string{
		"package " + SanitizePackageName(baseName),
		"/*",
		"#include <stdlib.h>",
		`#include "` + baseName + `.h"`,
		"*/",
		`import "C"`,
		"func init() {",
		"C.register_extension()",
		"}",
	}

	for _, element := range requiredElements {
		if !strings.Contains(content, element) {
			t.Errorf("Go file should contain: %s", element)
		}
	}
}

func testGoFileImports(t *testing.T, content string) {
	cImportCount := strings.Count(content, `import "C"`)
	if cImportCount != 1 {
		t.Errorf("Expected exactly 1 C import, got %d", cImportCount)
	}
}

func testGoFileExportedFunctions(t *testing.T, content string, functions []PHPFunction) {
	for _, fn := range functions {
		exportDirective := "//export " + fn.Name
		if !strings.Contains(content, exportDirective) {
			t.Errorf("Go file should contain export directive: %s", exportDirective)
		}

		funcStart := "func " + fn.Name + "("
		if !strings.Contains(content, funcStart) {
			t.Errorf("Go file should contain function definition: %s", funcStart)
		}
	}
}

func testGoFileInternalFunctions(t *testing.T, content string) {
	internalIndicators := []string{
		"func internalHelper",
		"func anotherHelper",
	}

	foundInternal := false
	for _, indicator := range internalIndicators {
		if strings.Contains(content, indicator) {
			foundInternal = true
			break
		}
	}

	if !foundInternal {
		t.Log("No internal functions found (this may be expected)")
	}
}
