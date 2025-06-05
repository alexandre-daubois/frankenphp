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
				"#include <php.h>",
				"void register_extension();",
				"extern zend_module_entry ext_module_entry;",
				"typedef struct go_value go_value;",
				"typedef struct go_string {",
				"size_t len;",
				"char *data;",
				"} go_string;",
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
			content, err := headerGen.buildContent()
			if err != nil {
				t.Fatalf("buildContent() failed: %v", err)
			}

			for _, expected := range tt.contains {
				if !strings.Contains(content, expected) {
					t.Errorf("Generated header content should contain '%s'\nGenerated:\n%s", expected, content)
				}
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
			content, err := headerGen.buildContent()
			if err != nil {
				t.Fatalf("buildContent() failed: %v", err)
			}

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

func TestHeaderGenerator_BasicStructure(t *testing.T) {
	generator := &Generator{BaseName: "structtest"}
	headerGen := HeaderGenerator{generator}
	content, err := headerGen.buildContent()
	if err != nil {
		t.Fatalf("buildContent() failed: %v", err)
	}

	expectedElements := []string{
		"#include <php.h>",
		"void register_extension();",
		"extern zend_module_entry ext_module_entry;",
		"typedef struct go_value go_value;",
		"typedef struct go_string {",
		"size_t len;",
		"char *data;",
		"} go_string;",
	}

	for _, element := range expectedElements {
		if !strings.Contains(content, element) {
			t.Errorf("Header should contain: %s", element)
		}
	}
}

func TestHeaderGenerator_CompleteStructure(t *testing.T) {
	generator := &Generator{BaseName: "complete_test"}
	headerGen := HeaderGenerator{generator}
	content, err := headerGen.buildContent()
	if err != nil {
		t.Fatalf("buildContent() failed: %v", err)
	}

	lines := strings.Split(content, "\n")

	if len(lines) < 5 {
		t.Error("Header file should have multiple lines")
	}

	var foundIfndef, foundDefine, foundEndif bool

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
	content, err := headerGen.buildContent()
	if err != nil {
		t.Fatalf("buildContent() failed: %v", err)
	}

	if !strings.Contains(content, "#ifndef __H") {
		t.Error("Header with empty basename should have __H guard")
	}

	if !strings.Contains(content, "#define __H") {
		t.Error("Header with empty basename should have __H define")
	}
}

func TestHeaderGenerator_ContentValidation(t *testing.T) {
	generator := &Generator{BaseName: "validation_test"}
	headerGen := HeaderGenerator{generator}
	content, err := headerGen.buildContent()
	if err != nil {
		t.Fatalf("buildContent() failed: %v", err)
	}

	if strings.Count(content, "#ifndef") != 1 {
		t.Error("Header should have exactly one #ifndef")
	}

	if strings.Count(content, "#define") != 1 {
		t.Error("Header should have exactly one #define")
	}

	if strings.Count(content, "#endif") != 1 {
		t.Error("Header should have exactly one #endif")
	}

	if strings.Contains(content, "{{") || strings.Contains(content, "}}") {
		t.Error("Generated header contains unresolved template syntax")
	}

	if !strings.Contains(content, "typedef struct go_string {") {
		t.Error("Header should contain go_string typedef")
	}

	if !strings.Contains(content, "size_t len;") {
		t.Error("Header should contain len field in go_string")
	}

	if !strings.Contains(content, "char *data;") {
		t.Error("Header should contain data field in go_string")
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
			content, err := headerGen.buildContent()
			if err != nil {
				t.Fatalf("buildContent() failed: %v", err)
			}

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

func TestHeaderGenerator_TemplateErrorHandling(t *testing.T) {
	generator := &Generator{BaseName: "error_test"}
	headerGen := HeaderGenerator{generator}

	_, err := headerGen.buildContent()
	if err != nil {
		t.Errorf("buildContent() should not fail with valid template: %v", err)
	}
}

func TestHeaderGenerator_GuardConsistency(t *testing.T) {
	baseName := "test_consistency"
	generator := &Generator{BaseName: baseName}
	headerGen := HeaderGenerator{generator}

	content1, err := headerGen.buildContent()
	if err != nil {
		t.Fatalf("First buildContent() failed: %v", err)
	}

	content2, err := headerGen.buildContent()
	if err != nil {
		t.Fatalf("Second buildContent() failed: %v", err)
	}

	if content1 != content2 {
		t.Error("Multiple calls to buildContent() should produce identical results")
	}
}

func TestHeaderGenerator_MinimalContent(t *testing.T) {
	generator := &Generator{BaseName: "minimal"}
	headerGen := HeaderGenerator{generator}
	content, err := headerGen.buildContent()
	if err != nil {
		t.Fatalf("buildContent() failed: %v", err)
	}

	essentialElements := []string{
		"#ifndef _MINIMAL_H",
		"#define _MINIMAL_H",
		"#include <php.h>",
		"void register_extension();",
		"extern zend_module_entry ext_module_entry;",
		"typedef struct go_value go_value;",
		"#endif",
	}

	for _, element := range essentialElements {
		if !strings.Contains(content, element) {
			t.Errorf("Minimal header should contain: %s", element)
		}
	}
}

func testHeaderBasicStructure(t *testing.T, content, baseName string) {
	headerGuard := strings.Map(func(r rune) rune {
		if r >= 'A' && r <= 'Z' || r >= 'a' && r <= 'z' || r >= '0' && r <= '9' {
			return r
		}
		return '_'
	}, baseName)
	headerGuard = strings.ToUpper(headerGuard) + "_H"

	requiredElements := []string{
		"#ifndef _" + headerGuard,
		"#define _" + headerGuard,
		"#include <php.h>",
		"void register_extension();",
		"extern zend_module_entry ext_module_entry;",
		"typedef struct go_value go_value;",
		"typedef struct go_string {",
		"size_t len;",
		"char *data;",
		"} go_string;",
		"#endif",
	}

	for _, element := range requiredElements {
		if !strings.Contains(content, element) {
			t.Errorf("Header file should contain: %s", element)
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
