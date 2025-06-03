package extgen

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestHeaderGenerator_Generate(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "header_generator_test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	generator := &Generator{
		BaseName: "test_extension",
		BuildDir: tmpDir,
	}

	headerGen := HeaderGenerator{generator}
	err = headerGen.generate()
	if err != nil {
		t.Fatalf("generate() failed: %v", err)
	}

	expectedFile := filepath.Join(tmpDir, "test_extension.h")
	if _, err := os.Stat(expectedFile); os.IsNotExist(err) {
		t.Errorf("Expected header file was not created: %s", expectedFile)
	}

	content, err := ReadFile(expectedFile)
	if err != nil {
		t.Fatalf("Failed to read generated header file: %v", err)
	}

	testHeaderBasicStructure(t, content, "test_extension")
	testHeaderFunctionDeclarations(t, content)
	testHeaderIncludeGuards(t, content, "TEST_EXTENSION_H")
}

func TestHeaderGenerator_BuildContent(t *testing.T) {
	tests := []struct {
		name     string
		baseName string
		contains []string
	}{
		{
			name:     "simple extension",
			baseName: "simple",
			contains: []string{
				"#ifndef _SIMPLE_H",
				"#define _SIMPLE_H",
				"#include <Zend/zend_types.h>",
				`#include "types.h"`,
				"void register_extension();",
				"void cleanup_go_value(go_value*);",
				"#endif",
			},
		},
		{
			name:     "extension with hyphens",
			baseName: "my-extension",
			contains: []string{
				"#ifndef _MY_EXTENSION_H",
				"#define _MY_EXTENSION_H",
				"void register_extension();",
				"#endif",
			},
		},
		{
			name:     "extension with underscores",
			baseName: "my_extension_name",
			contains: []string{
				"#ifndef _MY_EXTENSION_NAME_H",
				"#define _MY_EXTENSION_NAME_H",
				"void register_extension();",
				"#endif",
			},
		},
		{
			name:     "complex extension name",
			baseName: "complex.name-with_symbols",
			contains: []string{
				"#ifndef _COMPLEX_NAME_WITH_SYMBOLS_H",
				"#define _COMPLEX_NAME_WITH_SYMBOLS_H",
				"void register_extension();",
				"#endif",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			generator := &Generator{BaseName: tt.baseName}
			headerGen := HeaderGenerator{generator}
			content := headerGen.buildContent()

			for _, expected := range tt.contains {
				if !strings.Contains(content, expected) {
					t.Errorf("Generated header content should contain '%s'\nGenerated:\n%s", expected, content)
				}
			}

			headerGuard := strings.ToUpper(strings.ReplaceAll(tt.baseName, "-", "_"))
			headerGuard = strings.ReplaceAll(headerGuard, ".", "_") + "_H"

			expectedIfndef := "#ifndef _" + headerGuard
			expectedDefine := "#define _" + headerGuard

			if !strings.Contains(content, expectedIfndef) {
				t.Errorf("Header should contain proper #ifndef guard: %s", expectedIfndef)
			}

			if !strings.Contains(content, expectedDefine) {
				t.Errorf("Header should contain proper #define guard: %s", expectedDefine)
			}
		})
	}
}

func TestHeaderGenerator_HeaderGuardGeneration(t *testing.T) {
	tests := []struct {
		baseName      string
		expectedGuard string
	}{
		{"simple", "_SIMPLE_H"},
		{"my-extension", "_MY_EXTENSION_H"},
		{"complex.name", "_COMPLEX_NAME_H"},
		{"under_score", "_UNDER_SCORE_H"},
		{"MixedCase", "_MIXEDCASE_H"},
		{"123numeric", "_123NUMERIC_H"},
		{"special!@#chars", "_SPECIAL___CHARS_H"},
	}

	for _, tt := range tests {
		t.Run(tt.baseName, func(t *testing.T) {
			generator := &Generator{BaseName: tt.baseName}
			headerGen := HeaderGenerator{generator}
			content := headerGen.buildContent()

			expectedIfndef := "#ifndef " + tt.expectedGuard
			expectedDefine := "#define " + tt.expectedGuard

			if !strings.Contains(content, expectedIfndef) {
				t.Errorf("Expected #ifndef %s, but not found in content", tt.expectedGuard)
			}

			if !strings.Contains(content, expectedDefine) {
				t.Errorf("Expected #define %s, but not found in content", tt.expectedGuard)
			}
		})
	}
}

func TestHeaderGenerator_FunctionDeclarations(t *testing.T) {
	generator := &Generator{BaseName: "functest"}
	headerGen := HeaderGenerator{generator}
	content := headerGen.buildContent()

	expectedDeclarations := []string{
		"void register_extension();",
		"void cleanup_go_value(go_value*);",
		"void cleanup_go_array(go_array*);",
		"void cleanup_go_object(go_object*);",
		"void cleanup_go_nullable(go_nullable*);",
		"void cleanup_go_object_property(go_object_property*);",
		"void cleanup_go_array_element(go_array_element *);",
		"go_array* zval_to_go_array(zval *arr);",
		"go_object* zval_to_go_object(zval *obj);",
		"void go_array_to_zval(go_array *arr, zval *return_value);",
		"void go_object_to_zval(go_object *obj, zval *return_value);",
		"void register_all_classes();",
		"int get_property_visibility(zend_class_entry *ce, const char *property_name);",
		"void set_property_with_visibility(zval *object, zend_class_entry *ce, const char *property_name, zval *value);",
		"go_nullable* create_nullable_string(char* str, size_t len, int is_null);",
		"go_nullable* create_nullable_long(long val, int is_null);",
		"go_nullable* create_nullable_double(double val, int is_null);",
		"go_nullable* create_nullable_bool(int val, int is_null);",
		"go_nullable* create_nullable_array(zval* arr, int is_null);",
		"go_nullable* create_nullable_object(zval* obj, int is_null);",
	}

	for _, decl := range expectedDeclarations {
		if !strings.Contains(content, decl) {
			t.Errorf("Header should contain function declaration: %s", decl)
		}
	}
}

func TestHeaderGenerator_IncludeStatements(t *testing.T) {
	generator := &Generator{BaseName: "includetest"}
	headerGen := HeaderGenerator{generator}
	content := headerGen.buildContent()

	expectedIncludes := []string{
		"#include <Zend/zend_types.h>",
		`#include "types.h"`,
	}

	for _, include := range expectedIncludes {
		if !strings.Contains(content, include) {
			t.Errorf("Header should contain include: %s", include)
		}
	}
}

func TestHeaderGenerator_CompleteStructure(t *testing.T) {
	generator := &Generator{BaseName: "complete_test"}
	headerGen := HeaderGenerator{generator}
	content := headerGen.buildContent()

	lines := strings.Split(content, "\n")

	if len(lines) < 5 {
		t.Error("Header file should have multiple lines")
	}

	foundIfndef := false
	foundDefine := false
	foundEndif := false

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		if strings.HasPrefix(line, "#ifndef") && !foundIfndef {
			foundIfndef = true
		} else if strings.HasPrefix(line, "#define") && foundIfndef && !foundDefine {
			foundDefine = true
		} else if line == "#endif" {
			foundEndif = true
		}
	}

	if !foundIfndef {
		t.Error("Header should start with #ifndef guard")
	}

	if !foundDefine {
		t.Error("Header should have #define after #ifndef")
	}

	if !foundEndif {
		t.Error("Header should end with #endif")
	}
}

func TestHeaderGenerator_ErrorHandling(t *testing.T) {
	generator := &Generator{
		BaseName: "test",
		BuildDir: "/invalid/readonly/path",
	}

	headerGen := HeaderGenerator{generator}
	err := headerGen.generate()
	if err == nil {
		t.Error("Expected error when writing to invalid directory")
	}
}

func TestHeaderGenerator_EmptyBaseName(t *testing.T) {
	generator := &Generator{BaseName: ""}
	headerGen := HeaderGenerator{generator}
	content := headerGen.buildContent()

	if !strings.Contains(content, "#ifndef __H") {
		t.Error("Header with empty basename should have __H guard, got " + content)
	}

	if !strings.Contains(content, "#define __H") {
		t.Error("Header with empty basename should have _H define")
	}
}

func TestHeaderGenerator_ContentValidation(t *testing.T) {
	generator := &Generator{BaseName: "validation_test"}
	headerGen := HeaderGenerator{generator}
	content := headerGen.buildContent()

	if strings.Count(content, "#ifndef") != 1 {
		t.Error("Header should have exactly one #ifndef")
	}

	if strings.Count(content, "#define") != 1 {
		t.Error("Header should have exactly one #define")
	}

	if strings.Count(content, "#endif") != 1 {
		t.Error("Header should have exactly one #endif")
	}

	lines := strings.Split(content, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.Contains(line, "(") && strings.Contains(line, ")") && !strings.HasPrefix(line, "#") {
			if !strings.HasSuffix(line, ";") {
				t.Errorf("Function declaration should end with semicolon: %s", line)
			}
		}
	}

	for _, line := range lines {
		if strings.Contains(line, "void ") || strings.Contains(line, "go_") {
			openParens := strings.Count(line, "(")
			closeParens := strings.Count(line, ")")
			if openParens != closeParens {
				t.Errorf("Unbalanced parentheses in line: %s", line)
			}
		}
	}
}

func TestHeaderGenerator_SpecialCharacterHandling(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"normal", "NORMAL"},
		{"with-hyphens", "WITH_HYPHENS"},
		{"with.dots", "WITH_DOTS"},
		{"with_underscores", "WITH_UNDERSCORES"},
		{"MixedCASE", "MIXEDCASE"},
		{"123numbers", "123NUMBERS"},
		{"special!@#$%", "SPECIAL_____"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			generator := &Generator{BaseName: tt.input}
			headerGen := HeaderGenerator{generator}
			content := headerGen.buildContent()

			expectedGuard := "_" + tt.expected + "_H"
			expectedIfndef := "#ifndef " + expectedGuard
			expectedDefine := "#define " + expectedGuard

			if !strings.Contains(content, expectedIfndef) {
				t.Errorf("Expected #ifndef %s for input %s", expectedGuard, tt.input)
			}

			if !strings.Contains(content, expectedDefine) {
				t.Errorf("Expected #define %s for input %s", expectedGuard, tt.input)
			}
		})
	}
}

func testHeaderBasicStructure(t *testing.T, content, baseName string) {
	headerGuard := strings.ToUpper(strings.ReplaceAll(baseName, "-", "_")) + "_H"

	requiredElements := []string{
		"#ifndef _" + headerGuard,
		"#define _" + headerGuard,
		"#include <Zend/zend_types.h>",
		"#include \"types.h\"",
		"#endif",
	}

	for _, element := range requiredElements {
		if !strings.Contains(content, element) {
			t.Errorf("Header file should contain: %s", element)
		}
	}
}

func testHeaderFunctionDeclarations(t *testing.T, content string) {
	essentialFunctions := []string{
		"register_extension",
		"cleanup_go_value",
		"cleanup_go_array",
		"cleanup_go_object",
		"register_all_classes",
		"create_nullable_string",
		"create_nullable_long",
		"zval_to_go_array",
		"go_array_to_zval",
	}

	for _, fn := range essentialFunctions {
		if !strings.Contains(content, fn) {
			t.Errorf("Header should contain declaration for: %s", fn)
		}
	}
}

func testHeaderIncludeGuards(t *testing.T, content, expectedGuard string) {
	expectedIfndef := "#ifndef _" + expectedGuard
	expectedDefine := "#define _" + expectedGuard

	if !strings.Contains(content, expectedIfndef) {
		t.Errorf("Header should contain: %s", expectedIfndef)
	}

	if !strings.Contains(content, expectedDefine) {
		t.Errorf("Header should contain: %s", expectedDefine)
	}

	if !strings.Contains(content, "#endif") {
		t.Error("Header should end with #endif")
	}

	ifndefPos := strings.Index(content, expectedIfndef)
	definePos := strings.Index(content, expectedDefine)

	if ifndefPos >= definePos {
		t.Error("#ifndef should come before #define")
	}

	endifPos := strings.LastIndex(content, "#endif")
	if endifPos == -1 {
		t.Error("Header should end with #endif")
	}

	if endifPos <= definePos {
		t.Error("#endif should come after #define")
	}
}
