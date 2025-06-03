package extgen

import (
	"fmt"
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
				Properties: []ClassProperty{
					{Name: "id", Type: "int"},
					{Name: "name", Type: "string"},
				},
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
}

func TestStubGenerator_BuildContent(t *testing.T) {
	tests := []struct {
		name      string
		functions []PHPFunction
		classes   []PHPClass
		contains  []string
	}{
		{
			name:      "empty extension",
			functions: []PHPFunction{},
			classes:   []PHPClass{},
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
			classes: []PHPClass{},
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
					Properties: []ClassProperty{
						{Name: "id", Type: "int"},
						{Name: "name", Type: "string"},
					},
				},
			},
			contains: []string{
				"<?php",
				"/** @generate-class-entries */",
				"class TestClass {",
				"public int $id;",
				"public string $name;",
				"public function __construct() {}",
				"}",
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
					Properties: []ClassProperty{
						{Name: "success", Type: "bool"},
					},
				},
			},
			contains: []string{
				"function process(array $data): array {}",
				"class Result {",
				"public bool $success;",
				"public function __construct() {}",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			generator := &Generator{
				Functions: tt.functions,
				Classes:   tt.classes,
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
				Properties: []ClassProperty{
					{Name: "id", Type: "int"},
				},
			},
			contains: []string{
				"class SimpleClass {",
				"public int $id;",
				"public function __construct() {}",
				"}",
			},
		},
		{
			name: "class with nullable properties",
			class: PHPClass{
				Name: "NullableClass",
				Properties: []ClassProperty{
					{Name: "required", Type: "string", IsNullable: false},
					{Name: "optional", Type: "string", IsNullable: true},
				},
			},
			contains: []string{
				"class NullableClass {",
				"public string $required;",
				"public ?string $optional;",
				"public function __construct() {}",
			},
		},
		{
			name: "class with various property types",
			class: PHPClass{
				Name: "VariousTypes",
				Properties: []ClassProperty{
					{Name: "id", Type: "int"},
					{Name: "name", Type: "string"},
					{Name: "price", Type: "float"},
					{Name: "active", Type: "bool"},
					{Name: "tags", Type: "array"},
					{Name: "metadata", Type: "object"},
					{Name: "mixed_data", Type: "mixed"},
				},
			},
			contains: []string{
				"class VariousTypes {",
				"public int $id;",
				"public string $name;",
				"public float $price;",
				"public bool $active;",
				"public array $tags;",
				"public object $metadata;",
				"public mixed $mixedData;",
			},
		},
		{
			name: "class with no properties",
			class: PHPClass{
				Name:       "EmptyClass",
				Properties: []ClassProperty{},
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

func TestStubGenerator_PropertyNaming(t *testing.T) {
	tests := []struct {
		propertyName string
		expected     string
	}{
		{"ID", "public int $id;"},
		{"Name", "public string $name;"},
		{"UserID", "public int $userId;"},
		{"XMLData", "public string $xmldata;"},
		{"camelCase", "public string $camelCase;"},
		{"snake_case", "public string $snakeCase;"},
	}

	for _, tt := range tests {
		t.Run(tt.propertyName, func(t *testing.T) {
			class := PHPClass{
				Name: "TestClass",
				Properties: []ClassProperty{
					{Name: tt.propertyName, Type: "string"},
				},
			}

			if tt.propertyName == "ID" || tt.propertyName == "UserID" {
				class.Properties[0].Type = "int"
			}

			generator := &Generator{Classes: []PHPClass{class}}
			stubGen := StubGenerator{generator}
			content := stubGen.buildContent()

			if !strings.Contains(content, tt.expected) {
				t.Errorf("Property naming: expected '%s' in generated content\nGenerated:\n%s", tt.expected, content)
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
			Properties: []ClassProperty{
				{Name: "prop1", Type: "string"},
			},
		},
		{
			Name: "Class2",
			Properties: []ClassProperty{
				{Name: "prop2", Type: "int"},
				{Name: "prop3", Type: "bool"},
			},
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
			Properties: []ClassProperty{
				{Name: "id", Type: "int", IsNullable: false},
				{Name: "data", Type: "string", IsNullable: true},
				{Name: "metadata", Type: "array", IsNullable: true},
			},
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
		{";", "should contain semicolons"},
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

func TestStubGenerator_FileStructure(t *testing.T) {
	generator := &Generator{
		Functions: []PHPFunction{
			{Name: "testFunc", Signature: "testFunc(): void"},
		},
		Classes: []PHPClass{
			{
				Name: "TestClass",
				Properties: []ClassProperty{
					{Name: "prop", Type: "string"},
				},
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

func TestStubGenerator_PropertyTypeMapping(t *testing.T) {
	tests := []struct {
		goType      string
		phpType     string
		isNullable  bool
		expectedPHP string
	}{
		{"string", "string", false, "public string $"},
		{"string", "string", true, "public ?string $"},
		{"int", "int", false, "public int $"},
		{"int", "int", true, "public ?int $"},
		{"float", "float", false, "public float $"},
		{"float", "float", true, "public ?float $"},
		{"bool", "bool", false, "public bool $"},
		{"bool", "bool", true, "public ?bool $"},
		{"array", "array", false, "public array $"},
		{"array", "array", true, "public ?array $"},
		{"object", "object", false, "public object $"},
		{"object", "object", true, "public ?object $"},
		{"mixed", "mixed", false, "public mixed $"},
		{"mixed", "mixed", true, "public ?mixed $"},
	}

	for _, tt := range tests {
		nullableStr := "false"
		if tt.isNullable {
			nullableStr = "true"
		}

		t.Run(tt.goType+"_nullable_"+nullableStr, func(t *testing.T) {
			class := PHPClass{
				Name: "TypeTest",
				Properties: []ClassProperty{
					{
						Name:       "testProp",
						Type:       tt.phpType,
						GoType:     tt.goType,
						IsNullable: tt.isNullable,
					},
				},
			}

			generator := &Generator{Classes: []PHPClass{class}}
			stubGen := StubGenerator{generator}
			content := stubGen.buildContent()

			expectedDeclaration := tt.expectedPHP + "testProp;"
			if !strings.Contains(content, expectedDeclaration) {
				t.Errorf("Should contain property declaration: %s\nGenerated:\n%s", expectedDeclaration, content)
			}
		})
	}
}

// Helper functions for testing

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

		for _, prop := range class.Properties {
			nullable := ""
			if prop.IsNullable {
				nullable = "?"
			}
			expectedProp := fmt.Sprintf("public %s%s $%s;", nullable, prop.Type, strings.ToLower(prop.Name))
			if !strings.Contains(content, expectedProp) {
				t.Errorf("Class %s should contain property: %s", class.Name, expectedProp)
			}
		}

		if !strings.Contains(content, "}") {
			t.Errorf("Class %s should be properly closed", class.Name)
		}
	}
}
