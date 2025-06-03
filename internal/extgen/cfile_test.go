package extgen

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCFileGenerator_Generate(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "c_file_generator_test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	generator := &Generator{
		BaseName: "test_extension",
		BuildDir: tmpDir,
		Functions: []PHPFunction{
			{
				Name:       "simpleFunction",
				ReturnType: "string",
				Params: []Parameter{
					{Name: "input", Type: "string"},
				},
			},
			{
				Name:       "complexFunction",
				ReturnType: "array",
				Params: []Parameter{
					{Name: "data", Type: "string"},
					{Name: "count", Type: "int", IsNullable: true},
					{Name: "options", Type: "array", HasDefault: true, DefaultValue: "[]"},
				},
			},
		},
		Classes: []PHPClass{
			{
				Name:     "TestClass",
				GoStruct: "TestStruct",
				Properties: []ClassProperty{
					{Name: "id", Type: "int"},
					{Name: "name", Type: "string"},
				},
			},
		},
	}

	cGen := CFileGenerator{generator}
	err = cGen.generate()
	if err != nil {
		t.Fatalf("generate() failed: %v", err)
	}

	expectedFile := filepath.Join(tmpDir, "test_extension.c")
	if _, err := os.Stat(expectedFile); os.IsNotExist(err) {
		t.Errorf("Expected C file was not created: %s", expectedFile)
	}

	content, err := ReadFile(expectedFile)
	if err != nil {
		t.Fatalf("Failed to read generated C file: %v", err)
	}

	testCFileBasicStructure(t, content, "test_extension")
	testCFileFunctions(t, content, generator.Functions)
	testCFileClasses(t, content, generator.Classes)
}

func TestCFileGenerator_BuildContent(t *testing.T) {
	tests := []struct {
		name      string
		baseName  string
		functions []PHPFunction
		classes   []PHPClass
		contains  []string
	}{
		{
			name:     "empty extension",
			baseName: "empty",
			contains: []string{
				"#include <php.h>",
				"#include <Zend/zend_API.h>",
				`#include "empty.h"`,
				"PHP_MINIT_FUNCTION(empty)",
				"empty_module_entry",
				"register_extension()",
			},
		},
		{
			name:     "extension with functions only",
			baseName: "func_only",
			functions: []PHPFunction{
				{Name: "testFunc", ReturnType: "string"},
			},
			contains: []string{
				"PHP_FUNCTION(testFunc)",
				`#include "func_only.h"`,
				"func_only_module_entry",
			},
		},
		{
			name:     "extension with classes only",
			baseName: "class_only",
			classes: []PHPClass{
				{Name: "MyClass", GoStruct: "MyStruct"},
			},
			contains: []string{
				"register_all_classes()",
				"PHP_METHOD(MyClass, __construct)",
				`#include "class_only.h"`,
			},
		},
		{
			name:     "extension with functions and classes",
			baseName: "full",
			functions: []PHPFunction{
				{Name: "doSomething", ReturnType: "void"},
			},
			classes: []PHPClass{
				{Name: "FullClass", GoStruct: "FullStruct"},
			},
			contains: []string{
				"PHP_FUNCTION(doSomething)",
				"PHP_METHOD(FullClass, __construct)",
				"register_all_classes()",
				`#include "full.h"`,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			generator := &Generator{
				BaseName:  tt.baseName,
				Functions: tt.functions,
				Classes:   tt.classes,
			}

			cGen := CFileGenerator{generator}
			content := cGen.buildContent()

			for _, expected := range tt.contains {
				if !strings.Contains(content, expected) {
					t.Errorf("Generated C content should contain '%s'\nGenerated:\n%s", expected, content)
				}
			}
		})
	}
}

func TestCFileGenerator_GetTemplateContent(t *testing.T) {
	tests := []struct {
		baseName string
		contains []string
	}{
		{
			baseName: "myext",
			contains: []string{
				`#include "myext.h"`,
				`#include "myext_arginfo.h"`,
				"PHP_MINIT_FUNCTION(myext)",
				"myext_module_entry",
				"register_extension()",
			},
		},
		{
			baseName: "complex_name",
			contains: []string{
				`#include "complex_name.h"`,
				`#include "complex_name_arginfo.h"`,
				"PHP_MINIT_FUNCTION(complex_name)",
				"complex_name_module_entry",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.baseName, func(t *testing.T) {
			generator := &Generator{BaseName: tt.baseName}
			cGen := CFileGenerator{generator}
			content := cGen.getTemplateContent()

			for _, expected := range tt.contains {
				if !strings.Contains(content, expected) {
					t.Errorf("Template content should contain '%s'", expected)
				}
			}
		})
	}
}

func TestCFileIntegrationWithGenerators(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "c_integration_test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	functions := []PHPFunction{
		{
			Name:             "processData",
			ReturnType:       "array",
			IsReturnNullable: true,
			Params: []Parameter{
				{Name: "input", Type: "string"},
				{Name: "options", Type: "array", HasDefault: true, DefaultValue: "[]"},
				{Name: "callback", Type: "object", IsNullable: true},
			},
		},
		{
			Name:       "validateInput",
			ReturnType: "bool",
			Params: []Parameter{
				{Name: "data", Type: "string", IsNullable: true},
				{Name: "strict", Type: "bool", HasDefault: true, DefaultValue: "false"},
			},
		},
	}

	classes := []PHPClass{
		{
			Name:     "DataProcessor",
			GoStruct: "DataProcessorStruct",
			Properties: []ClassProperty{
				{Name: "mode", Type: "string"},
				{Name: "timeout", Type: "int", IsNullable: true},
				{Name: "options", Type: "array"},
			},
		},
		{
			Name:     "Result",
			GoStruct: "ResultStruct",
			Properties: []ClassProperty{
				{Name: "success", Type: "bool"},
				{Name: "data", Type: "mixed", IsNullable: true},
				{Name: "errors", Type: "array"},
			},
		},
	}

	generator := &Generator{
		BaseName:  "integration_test",
		BuildDir:  tmpDir,
		Functions: functions,
		Classes:   classes,
	}

	cGen := CFileGenerator{generator}
	err = cGen.generate()
	if err != nil {
		t.Fatalf("generate() failed: %v", err)
	}

	content, err := ReadFile(filepath.Join(tmpDir, "integration_test.c"))
	if err != nil {
		t.Fatalf("Failed to read generated file: %v", err)
	}

	for _, fn := range functions {
		expectedFunc := "PHP_FUNCTION(" + fn.Name + ")"
		if !strings.Contains(content, expectedFunc) {
			t.Errorf("Generated C file should contain function: %s", expectedFunc)
		}
	}

	for _, class := range classes {
		expectedMethod := "PHP_METHOD(" + class.Name + ", __construct)"
		if !strings.Contains(content, expectedMethod) {
			t.Errorf("Generated C file should contain class method: %s", expectedMethod)
		}
	}

	if !strings.Contains(content, "register_all_classes()") {
		t.Error("Generated C file should contain class registration call")
	}

	expectedStructures := []string{
		"static int (*original_php_register_internal_extensions_func)(void) = NULL;",
		"integration_test_module_entry",
		"register_internal_extensions(void)",
		"register_extension()",
	}

	for _, structure := range expectedStructures {
		if !strings.Contains(content, structure) {
			t.Errorf("Generated C file should contain: %s", structure)
		}
	}
}

func TestCFileErrorHandling(t *testing.T) {
	// Test with invalid build directory
	generator := &Generator{
		BaseName: "test",
		BuildDir: "/invalid/readonly/path",
		Functions: []PHPFunction{
			{Name: "test", ReturnType: "void"},
		},
	}

	cGen := CFileGenerator{generator}
	err := cGen.generate()
	if err == nil {
		t.Error("Expected error when writing to invalid directory")
	}
}

func TestCFileSpecialCharacters(t *testing.T) {
	tests := []struct {
		baseName string
		expected string
	}{
		{"simple", "simple"},
		{"my_extension", "my_extension"},
		{"ext-with-dashes", "ext-with-dashes"},
	}

	for _, tt := range tests {
		t.Run(tt.baseName, func(t *testing.T) {
			generator := &Generator{
				BaseName: tt.baseName,
				Functions: []PHPFunction{
					{Name: "test", ReturnType: "void"},
				},
			}

			cGen := CFileGenerator{generator}
			content := cGen.buildContent()

			expectedInclude := "#include \"" + tt.expected + ".h\""
			if !strings.Contains(content, expectedInclude) {
				t.Errorf("Content should contain include: %s", expectedInclude)
			}
		})
	}
}

func testCFileBasicStructure(t *testing.T, content, baseName string) {
	requiredElements := []string{
		"#include <php.h>",
		"#include <Zend/zend_API.h>",
		`#include "_cgo_export.h"`,
		`#include "` + baseName + `.h"`,
		`#include "` + baseName + `_arginfo.h"`,
		"static int (*original_php_register_internal_extensions_func)(void) = NULL;",
		"PHP_MINIT_FUNCTION(" + baseName + ")",
		baseName + "_module_entry",
		"register_internal_extensions(void)",
		"register_extension()",
	}

	for _, element := range requiredElements {
		if !strings.Contains(content, element) {
			t.Errorf("C file should contain: %s", element)
		}
	}
}

func testCFileFunctions(t *testing.T, content string, functions []PHPFunction) {
	for _, fn := range functions {
		phpFunc := "PHP_FUNCTION(" + fn.Name + ")"
		if !strings.Contains(content, phpFunc) {
			t.Errorf("C file should contain function declaration: %s", phpFunc)
		}
	}
}

func testCFileClasses(t *testing.T, content string, classes []PHPClass) {
	if len(classes) == 0 {
		if !strings.Contains(content, "void register_all_classes() {\n    // No classes to register\n}") {
			t.Error("C file should contain empty class registration for no classes")
		}
		return
	}

	if !strings.Contains(content, "void register_all_classes() {") {
		t.Error("C file should contain register_all_classes function")
	}

	for _, class := range classes {
		constructor := "PHP_METHOD(" + class.Name + ", __construct)"
		if !strings.Contains(content, constructor) {
			t.Errorf("C file should contain constructor: %s", constructor)
		}

		className := SanitizePackageName(strings.ToLower(class.Name))
		classEntry := "static zend_class_entry *" + className + "_ce = NULL;"
		if !strings.Contains(content, classEntry) {
			t.Errorf("C file should contain class entry variable: %s", classEntry)
		}

		registerCall := className + "_ce = register_class_" + class.Name + "();"
		if !strings.Contains(content, registerCall) {
			t.Errorf("C file should contain class registration: %s", registerCall)
		}
	}
}

func TestCFileContentValidation(t *testing.T) {
	generator := &Generator{
		BaseName: "syntax_test",
		Functions: []PHPFunction{
			{
				Name:       "testFunction",
				ReturnType: "string",
				Params: []Parameter{
					{Name: "param", Type: "string"},
				},
			},
		},
	}

	cGen := CFileGenerator{generator}
	content := cGen.buildContent()

	syntaxElements := []string{
		"{", "}", "(", ")", ";",
		"static", "void", "int",
		"#include",
	}

	for _, element := range syntaxElements {
		if !strings.Contains(content, element) {
			t.Errorf("Generated C content should contain basic C syntax: %s", element)
		}
	}

	openBraces := strings.Count(content, "{")
	closeBraces := strings.Count(content, "}")
	if openBraces != closeBraces {
		t.Errorf("Unbalanced braces in generated C code: %d open, %d close", openBraces, closeBraces)
	}

	if strings.Contains(content, ";;") {
		t.Error("Generated C code contains double semicolons")
	}

	if strings.Contains(content, "{{") && !strings.Contains(content, "struct") {
		t.Error("Generated C code may contain syntax errors (double braces)")
	}
}
