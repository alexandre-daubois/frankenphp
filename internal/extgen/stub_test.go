package extgen

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestStubGenerator_Generate(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "stub_generator_test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	generator := &Generator{
		BaseName: "test_extension",
		BuildDir: tmpDir,
		Functions: []PHPFunction{
			{
				Name:      "greet",
				Signature: "greet(string $name): string",
				Params: []Parameter{
					{Name: "name", Type: "string"},
				},
				ReturnType: "string",
			},
			{
				Name:      "calculate",
				Signature: "calculate(int $a, int $b): int",
				Params: []Parameter{
					{Name: "a", Type: "int"},
					{Name: "b", Type: "int"},
				},
				ReturnType: "int",
			},
		},
		Classes: []PHPClass{
			{
				Name:     "User",
				GoStruct: "UserStruct",
			},
		},
		Constants: []PHPConstant{
			{
				Name:  "GLOBAL_CONST",
				Value: "42",
				Type:  "int",
			},
			{
				Name:      "USER_STATUS_ACTIVE",
				Value:     "1",
				Type:      "int",
				ClassName: "User",
			},
		},
	}

	stubGen := StubGenerator{generator}
	err = stubGen.generate()
	if err != nil {
		t.Fatalf("generate() failed: %v", err)
	}

	expectedFile := filepath.Join(tmpDir, "test_extension.stub.php")
	if _, err := os.Stat(expectedFile); os.IsNotExist(err) {
		t.Errorf("Expected stub file was not created: %s", expectedFile)
	}

	content, err := ReadFile(expectedFile)
	if err != nil {
		t.Fatalf("Failed to read generated stub file: %v", err)
	}

	testStubBasicStructure(t, content)
	testStubFunctions(t, content, generator.Functions)
	testStubClasses(t, content, generator.Classes)
	testStubConstants(t, content, generator.Constants)
}

func TestStubGenerator_BuildContent(t *testing.T) {
	tests := []struct {
		name      string
		functions []PHPFunction
		classes   []PHPClass
		constants []PHPConstant
		contains  []string
	}{
		{
			name:      "empty extension",
			functions: []PHPFunction{},
			classes:   []PHPClass{},
			constants: []PHPConstant{},
			contains: []string{
				"<?php",
				"/** @generate-class-entries */",
			},
		},
		{
			name: "functions only",
			functions: []PHPFunction{
				{
					Name:      "testFunc",
					Signature: "testFunc(string $param): bool",
				},
			},
			classes:   []PHPClass{},
			constants: []PHPConstant{},
			contains: []string{
				"<?php",
				"/** @generate-class-entries */",
				"function testFunc(string $param): bool {}",
			},
		},
		{
			name:      "classes only",
			functions: []PHPFunction{},
			classes: []PHPClass{
				{
					Name: "TestClass",
				},
			},
			constants: []PHPConstant{},
			contains: []string{
				"<?php",
				"/** @generate-class-entries */",
				"class TestClass {",
				"public function __construct() {}",
				"}",
			},
		},
		{
			name:      "constants only",
			functions: []PHPFunction{},
			classes:   []PHPClass{},
			constants: []PHPConstant{
				{
					Name:  "GLOBAL_CONST",
					Value: "\"test\"",
					Type:  "string",
				},
			},
			contains: []string{
				"<?php",
				"/** @generate-class-entries */",
				"const GLOBAL_CONST = \"test\";",
			},
		},
		{
			name: "functions and classes",
			functions: []PHPFunction{
				{
					Name:      "process",
					Signature: "process(array $data): array",
				},
			},
			classes: []PHPClass{
				{
					Name: "Result",
				},
			},
			constants: []PHPConstant{},
			contains: []string{
				"function process(array $data): array {}",
				"class Result {",
				"public function __construct() {}",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			generator := &Generator{
				Functions: tt.functions,
				Classes:   tt.classes,
				Constants: tt.constants,
			}

			stubGen := StubGenerator{generator}
			content := stubGen.buildContent()

			for _, expected := range tt.contains {
				if !strings.Contains(content, expected) {
					t.Errorf("Generated stub content should contain '%s'\nGenerated:\n%s", expected, content)
				}
			}
		})
	}
}

func TestStubGenerator_FunctionSignatures(t *testing.T) {
	tests := []struct {
		name     string
		function PHPFunction
		expected string
	}{
		{
			name: "simple function",
			function: PHPFunction{
				Name:      "test",
				Signature: "test(): void",
			},
			expected: "function test(): void {}",
		},
		{
			name: "function with parameters",
			function: PHPFunction{
				Name:      "greet",
				Signature: "greet(string $name): string",
			},
			expected: "function greet(string $name): string {}",
		},
		{
			name: "function with nullable return",
			function: PHPFunction{
				Name:      "findUser",
				Signature: "findUser(int $id): ?object",
			},
			expected: "function findUser(int $id): ?object {}",
		},
		{
			name: "complex function signature",
			function: PHPFunction{
				Name:      "process",
				Signature: "process(array $data, ?string $prefix = null, bool $strict = false): ?array",
			},
			expected: "function process(array $data, ?string $prefix = null, bool $strict = false): ?array {}",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			generator := &Generator{
				Functions: []PHPFunction{tt.function},
			}

			stubGen := StubGenerator{generator}
			content := stubGen.buildContent()

			if !strings.Contains(content, tt.expected) {
				t.Errorf("Generated content should contain function signature:\nExpected: %s\nGenerated:\n%s", tt.expected, content)
			}
		})
	}
}

func TestStubGenerator_ClassGeneration(t *testing.T) {
	tests := []struct {
		name     string
		class    PHPClass
		contains []string
	}{
		{
			name: "simple class",
			class: PHPClass{
				Name: "SimpleClass",
			},
			contains: []string{
				"class SimpleClass {",
				"public function __construct() {}",
				"}",
			},
		},
		{
			name: "class with no properties",
			class: PHPClass{
				Name: "EmptyClass",
			},
			contains: []string{
				"class EmptyClass {",
				"public function __construct() {}",
				"}",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			generator := &Generator{
				Classes: []PHPClass{tt.class},
			}

			stubGen := StubGenerator{generator}
			content := stubGen.buildContent()

			for _, expected := range tt.contains {
				if !strings.Contains(content, expected) {
					t.Errorf("Generated content should contain '%s'\nGenerated:\n%s", expected, content)
				}
			}
		})
	}
}

func TestStubGenerator_MultipleItems(t *testing.T) {
	functions := []PHPFunction{
		{
			Name:      "func1",
			Signature: "func1(): void",
		},
		{
			Name:      "func2",
			Signature: "func2(string $param): bool",
		},
		{
			Name:      "func3",
			Signature: "func3(int $a, int $b): int",
		},
	}

	classes := []PHPClass{
		{
			Name: "Class1",
		},
		{
			Name: "Class2",
		},
	}

	generator := &Generator{
		Functions: functions,
		Classes:   classes,
	}

	stubGen := StubGenerator{generator}
	content := stubGen.buildContent()

	for _, fn := range functions {
		expectedFunc := "function " + fn.Name
		if !strings.Contains(content, expectedFunc) {
			t.Errorf("Should contain function: %s", expectedFunc)
		}
	}

	for _, class := range classes {
		expectedClass := "class " + class.Name
		if !strings.Contains(content, expectedClass) {
			t.Errorf("Should contain class: %s", expectedClass)
		}
	}

	funcPos := strings.Index(content, "function func1")
	classPos := strings.Index(content, "class Class1")

	if funcPos == -1 || classPos == -1 {
		t.Error("Both functions and classes should be present")
	}

	if funcPos >= classPos {
		t.Error("Functions should appear before classes in the stub file")
	}
}

func TestStubGenerator_ErrorHandling(t *testing.T) {
	generator := &Generator{
		BaseName: "test",
		BuildDir: "/invalid/readonly/path",
		Functions: []PHPFunction{
			{Name: "test", Signature: "test(): void"},
		},
	}

	stubGen := StubGenerator{generator}
	err := stubGen.generate()
	if err == nil {
		t.Error("Expected error when writing to invalid directory")
	}
}

func TestStubGenerator_EmptyContent(t *testing.T) {
	generator := &Generator{
		Functions: []PHPFunction{},
		Classes:   []PHPClass{},
	}

	stubGen := StubGenerator{generator}
	content := stubGen.buildContent()

	expectedMinimal := []string{
		"<?php",
		"/** @generate-class-entries */",
	}

	for _, expected := range expectedMinimal {
		if !strings.Contains(content, expected) {
			t.Errorf("Even empty content should contain: %s", expected)
		}
	}

	if strings.Contains(content, "function ") {
		t.Error("Empty stub should not contain function declarations")
	}

	if strings.Contains(content, "class ") {
		t.Error("Empty stub should not contain class declarations")
	}
}

func TestStubGenerator_PHPSyntaxValidation(t *testing.T) {
	functions := []PHPFunction{
		{
			Name:      "complexFunc",
			Signature: "complexFunc(?string $name = null, array $options = [], bool $strict = false): ?object",
		},
	}

	classes := []PHPClass{
		{
			Name: "ComplexClass",
		},
	}

	generator := &Generator{
		Functions: functions,
		Classes:   classes,
	}

	stubGen := StubGenerator{generator}
	content := stubGen.buildContent()

	syntaxChecks := []struct {
		element string
		reason  string
	}{
		{"<?php", "should start with PHP opening tag"},
		{"{", "should contain opening braces"},
		{"}", "should contain closing braces"},
		{"public", "should use proper visibility"},
		{"function", "should contain function keyword"},
		{"class", "should contain class keyword"},
	}

	for _, check := range syntaxChecks {
		if !strings.Contains(content, check.element) {
			t.Errorf("Generated PHP %s", check.reason)
		}
	}

	openBraces := strings.Count(content, "{")
	closeBraces := strings.Count(content, "}")
	if openBraces != closeBraces {
		t.Errorf("Unbalanced braces in PHP: %d open, %d close", openBraces, closeBraces)
	}

	expectedSig := "function complexFunc(?string $name = null, array $options = [], bool $strict = false): ?object {}"
	if !strings.Contains(content, expectedSig) {
		t.Errorf("Complex function signature should be preserved exactly")
	}
}

func TestStubGenerator_ClassConstants(t *testing.T) {
	tests := []struct {
		name      string
		classes   []PHPClass
		constants []PHPConstant
		contains  []string
	}{
		{
			name: "class with constants",
			classes: []PHPClass{
				{Name: "MyClass"},
			},
			constants: []PHPConstant{
				{
					Name:      "STATUS_ACTIVE",
					Value:     "1",
					Type:      "int",
					ClassName: "MyClass",
				},
				{
					Name:      "STATUS_INACTIVE",
					Value:     "0",
					Type:      "int",
					ClassName: "MyClass",
				},
			},
			contains: []string{
				"class MyClass {",
				"public const STATUS_ACTIVE = 1;",
				"public const STATUS_INACTIVE = 0;",
				"public function __construct() {}",
			},
		},
		{
			name: "class with iota constants",
			classes: []PHPClass{
				{Name: "StatusClass"},
			},
			constants: []PHPConstant{
				{
					Name:      "FIRST",
					Value:     "0",
					Type:      "int",
					IsIota:    true,
					ClassName: "StatusClass",
				},
				{
					Name:      "SECOND",
					Value:     "1",
					Type:      "int",
					IsIota:    true,
					ClassName: "StatusClass",
				},
			},
			contains: []string{
				"class StatusClass {",
				"public const FIRST = UNKNOWN;",
				"public const SECOND = UNKNOWN;",
				"@cvalue FIRST",
				"@cvalue SECOND",
			},
		},
		{
			name: "global and class constants",
			classes: []PHPClass{
				{Name: "TestClass"},
			},
			constants: []PHPConstant{
				{
					Name:  "GLOBAL_CONST",
					Value: "\"global\"",
					Type:  "string",
				},
				{
					Name:      "CLASS_CONST",
					Value:     "42",
					Type:      "int",
					ClassName: "TestClass",
				},
			},
			contains: []string{
				"const GLOBAL_CONST = \"global\";",
				"class TestClass {",
				"public const CLASS_CONST = 42;",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			generator := &Generator{
				Classes:   tt.classes,
				Constants: tt.constants,
			}

			stubGen := StubGenerator{generator}
			content := stubGen.buildContent()

			for _, expected := range tt.contains {
				if !strings.Contains(content, expected) {
					t.Errorf("Generated content should contain '%s'\nGenerated:\n%s", expected, content)
				}
			}
		})
	}
}

func TestStubGenerator_FileStructure(t *testing.T) {
	generator := &Generator{
		Functions: []PHPFunction{
			{Name: "testFunc", Signature: "testFunc(): void"},
		},
		Classes: []PHPClass{
			{
				Name: "TestClass",
			},
		},
	}

	stubGen := StubGenerator{generator}
	content := stubGen.buildContent()

	lines := strings.Split(content, "\n")

	if len(lines) < 3 {
		t.Error("Stub file should have multiple lines")
	}

	if strings.TrimSpace(lines[0]) != "<?php" {
		t.Error("First line should be <?php opening tag")
	}

	foundGenerateDirective := false
	for _, line := range lines {
		if strings.Contains(line, "@generate-class-entries") {
			foundGenerateDirective = true
			break
		}
	}

	if !foundGenerateDirective {
		t.Error("Should contain @generate-class-entries directive")
	}

	contentStr := strings.Join(lines, "\n")
	if !strings.Contains(contentStr, "\n\n") {
		t.Error("Should have proper spacing between sections")
	}
}

func testStubBasicStructure(t *testing.T, content string) {
	requiredElements := []string{
		"<?php",
		"/** @generate-class-entries */",
	}

	for _, element := range requiredElements {
		if !strings.Contains(content, element) {
			t.Errorf("Stub file should contain: %s", element)
		}
	}

	lines := strings.Split(content, "\n")
	if len(lines) > 0 && strings.TrimSpace(lines[0]) != "<?php" {
		t.Error("Stub file should start with <?php")
	}
}

func testStubFunctions(t *testing.T, content string, functions []PHPFunction) {
	for _, fn := range functions {
		expectedFunc := "function " + fn.Signature + " {}"
		if !strings.Contains(content, expectedFunc) {
			t.Errorf("Stub should contain function: %s", expectedFunc)
		}
	}
}

func testStubClasses(t *testing.T, content string, classes []PHPClass) {
	for _, class := range classes {
		expectedClass := "class " + class.Name + " {"
		if !strings.Contains(content, expectedClass) {
			t.Errorf("Stub should contain class: %s", expectedClass)
		}

		expectedConstructor := "public function __construct() {}"
		if !strings.Contains(content, expectedConstructor) {
			t.Errorf("Class %s should have constructor", class.Name)
		}

		if !strings.Contains(content, "}") {
			t.Errorf("Class %s should be properly closed", class.Name)
		}
	}
}

func testStubConstants(t *testing.T, content string, constants []PHPConstant) {
	for _, constant := range constants {
		if constant.ClassName == "" {
			if constant.IsIota {
				expectedConst := "const " + constant.Name + " = UNKNOWN;"
				if !strings.Contains(content, expectedConst) {
					t.Errorf("Stub should contain iota constant: %s", expectedConst)
				}
			} else {
				expectedConst := "const " + constant.Name + " = " + constant.Value + ";"
				if !strings.Contains(content, expectedConst) {
					t.Errorf("Stub should contain constant: %s", expectedConst)
				}
			}
		} else {
			if constant.IsIota {
				expectedConst := "public const " + constant.Name + " = UNKNOWN;"
				if !strings.Contains(content, expectedConst) {
					t.Errorf("Stub should contain class iota constant: %s", expectedConst)
				}
			} else {
				expectedConst := "public const " + constant.Name + " = " + constant.Value + ";"
				if !strings.Contains(content, expectedConst) {
					t.Errorf("Stub should contain class constant: %s", expectedConst)
				}
			}
		}
	}
}
