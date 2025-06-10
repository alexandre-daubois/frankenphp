package extgen

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestConstantsIntegration(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.go")

	content := `package main

//export_php:const
const STATUS_OK = iota

//export_php:const
const MAX_CONNECTIONS = 100

//export_php:const: function test(): void
func Test() {
    // Implementation
}

func main() {}
`

	err := os.WriteFile(testFile, []byte(content), 0644)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	generator := &Generator{
		BaseName:   "testext",
		SourceFile: testFile,
		BuildDir:   filepath.Join(tmpDir, "build"),
	}

	err = generator.parseSource()
	if err != nil {
		t.Fatalf("Failed to parse source: %v", err)
	}

	if len(generator.Constants) != 2 {
		t.Errorf("Expected 2 constants, got %d", len(generator.Constants))
	}

	expectedConstants := map[string]struct {
		Value  string
		IsIota bool
	}{
		"STATUS_OK":       {"0", true},
		"MAX_CONNECTIONS": {"100", false},
	}

	for _, constant := range generator.Constants {
		expected, exists := expectedConstants[constant.Name]
		if !exists {
			t.Errorf("Unexpected constant: %s", constant.Name)
			continue
		}

		if constant.Value != expected.Value {
			t.Errorf("Constant %s: expected value %s, got %s", constant.Name, expected.Value, constant.Value)
		}
		if constant.IsIota != expected.IsIota {
			t.Errorf("Constant %s: expected IsIota %v, got %v", constant.Name, expected.IsIota, constant.IsIota)
		}
	}

	err = generator.setupBuildDirectory()
	if err != nil {
		t.Fatalf("Failed to setup build directory: %v", err)
	}

	err = generator.generateStubFile()
	if err != nil {
		t.Fatalf("Failed to generate stub file: %v", err)
	}

	stubPath := filepath.Join(generator.BuildDir, generator.BaseName+".stub.php")
	stubContent, err := os.ReadFile(stubPath)
	if err != nil {
		t.Fatalf("Failed to read stub file: %v", err)
	}

	stubStr := string(stubContent)

	if !strings.Contains(stubStr, "* @cvalue") {
		t.Error("Stub does not contain @cvalue annotation for iota constant")
	}
	if !strings.Contains(stubStr, "const STATUS_OK = UNKNOWN;") {
		t.Error("Stub does not contain STATUS_OK constant with UNKNOWN value")
	}

	if !strings.Contains(stubStr, "const MAX_CONNECTIONS = 100;") {
		t.Error("Stub does not contain MAX_CONNECTIONS constant with explicit value")
	}

	err = generator.generateCFile()
	if err != nil {
		t.Fatalf("Failed to generate C file: %v", err)
	}

	cPath := filepath.Join(generator.BuildDir, generator.BaseName+".c")
	cContent, err := os.ReadFile(cPath)
	if err != nil {
		t.Fatalf("Failed to read C file: %v", err)
	}

	cStr := string(cContent)

	if !strings.Contains(cStr, `REGISTER_LONG_CONSTANT("STATUS_OK", STATUS_OK, CONST_CS | CONST_PERSISTENT);`) {
		t.Error("C file does not contain STATUS_OK registration")
	}
	if !strings.Contains(cStr, `REGISTER_LONG_CONSTANT("MAX_CONNECTIONS", 100, CONST_CS | CONST_PERSISTENT);`) {
		t.Error("C file does not contain MAX_CONNECTIONS registration")
	}
}

func TestConstantsIntegrationOctal(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.go")

	content := `package main

//export_php:const
const FILE_PERM = 0o755

//export_php:const
const OTHER_PERM = 0o644

//export_php:const
const REGULAR_INT = 42

func main() {}
`

	err := os.WriteFile(testFile, []byte(content), 0644)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	generator := &Generator{
		BaseName:   "octalstest",
		SourceFile: testFile,
		BuildDir:   filepath.Join(tmpDir, "build"),
	}

	err = generator.parseSource()
	if err != nil {
		t.Fatalf("Failed to parse source: %v", err)
	}

	if len(generator.Constants) != 3 {
		t.Errorf("Expected 3 constants, got %d", len(generator.Constants))
	}

	// Verify CValue conversion
	for _, constant := range generator.Constants {
		switch constant.Name {
		case "FILE_PERM":
			if constant.Value != "0o755" {
				t.Errorf("Expected FILE_PERM value to be '0o755', got '%s'", constant.Value)
			}
			if constant.CValue() != "493" {
				t.Errorf("Expected FILE_PERM CValue to be '493', got '%s'", constant.CValue())
			}
		case "OTHER_PERM":
			if constant.Value != "0o644" {
				t.Errorf("Expected OTHER_PERM value to be '0o644', got '%s'", constant.Value)
			}
			if constant.CValue() != "420" {
				t.Errorf("Expected OTHER_PERM CValue to be '420', got '%s'", constant.CValue())
			}
		case "REGULAR_INT":
			if constant.Value != "42" {
				t.Errorf("Expected REGULAR_INT value to be '42', got '%s'", constant.Value)
			}
			if constant.CValue() != "42" {
				t.Errorf("Expected REGULAR_INT CValue to be '42', got '%s'", constant.CValue())
			}
		}
	}

	err = generator.setupBuildDirectory()
	if err != nil {
		t.Fatalf("Failed to setup build directory: %v", err)
	}

	// Test C file generation
	err = generator.generateCFile()
	if err != nil {
		t.Fatalf("Failed to generate C file: %v", err)
	}

	cPath := filepath.Join(generator.BuildDir, generator.BaseName+".c")
	cContent, err := os.ReadFile(cPath)
	if err != nil {
		t.Fatalf("Failed to read C file: %v", err)
	}

	cStr := string(cContent)

	// Verify C file uses decimal values for octal constants
	if !strings.Contains(cStr, `REGISTER_LONG_CONSTANT("FILE_PERM", 493, CONST_CS | CONST_PERSISTENT);`) {
		t.Error("C file does not contain FILE_PERM registration with decimal value 493")
	}
	if !strings.Contains(cStr, `REGISTER_LONG_CONSTANT("OTHER_PERM", 420, CONST_CS | CONST_PERSISTENT);`) {
		t.Error("C file does not contain OTHER_PERM registration with decimal value 420")
	}
	if !strings.Contains(cStr, `REGISTER_LONG_CONSTANT("REGULAR_INT", 42, CONST_CS | CONST_PERSISTENT);`) {
		t.Error("C file does not contain REGULAR_INT registration with value 42")
	}

	// Test header file generation
	err = generator.generateHeaderFile()
	if err != nil {
		t.Fatalf("Failed to generate header file: %v", err)
	}

	hPath := filepath.Join(generator.BuildDir, generator.BaseName+".h")
	hContent, err := os.ReadFile(hPath)
	if err != nil {
		t.Fatalf("Failed to read header file: %v", err)
	}

	hStr := string(hContent)

	// Verify header file uses decimal values for octal constants in #define
	if !strings.Contains(hStr, "#define FILE_PERM 493") {
		t.Error("Header file does not contain FILE_PERM #define with decimal value 493")
	}
	if !strings.Contains(hStr, "#define OTHER_PERM 420") {
		t.Error("Header file does not contain OTHER_PERM #define with decimal value 420")
	}
	if !strings.Contains(hStr, "#define REGULAR_INT 42") {
		t.Error("Header file does not contain REGULAR_INT #define with value 42")
	}
}
