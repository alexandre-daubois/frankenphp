package extgen

import (
	"os"
	"testing"
)

func TestConstantParser(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected int
	}{
		{
			name: "single constant",
			input: `package main

//export_php:const
const MyConstant = "test_value"`,
			expected: 1,
		},
		{
			name: "multiple constants",
			input: `package main

//export_php:const
const FirstConstant = "first"

//export_php:const
const SecondConstant = 42

//export_php:const
const ThirdConstant = true`,
			expected: 3,
		},
		{
			name: "iota constant",
			input: `package main

//export_php:const
const IotaConstant = iota`,
			expected: 1,
		},
		{
			name: "mixed constants and iota",
			input: `package main

//export_php:const
const StringConst = "hello"

//export_php:const
const IotaConst = iota

//export_php:const
const IntConst = 123`,
			expected: 3,
		},
		{
			name: "no php constants",
			input: `package main

const RegularConstant = "not exported"

func someFunction() {
	// Just regular code
}`,
			expected: 0,
		},
		{
			name: "constant with complex value",
			input: `package main

//export_php:const
const ComplexConstant = "string with spaces and symbols !@#$%"`,
			expected: 1,
		},
		{
			name: "directive without constant",
			input: `package main

//export_php:const
var notAConstant = "this is a variable"`,
			expected: 0,
		},
		{
			name: "mixed export and non-export constants",
			input: `package main

const RegularConst = "regular"

//export_php:const
const ExportedConst = "exported"

const AnotherRegular = 456

//export_php:const
const AnotherExported = 789`,
			expected: 2,
		},
		{
			name: "numeric constants",
			input: `package main

//export_php:const
const IntConstant = 42

//export_php:const
const FloatConstant = 3.14

//export_php:const
const HexConstant = 0xFF`,
			expected: 3,
		},
		{
			name: "boolean constants",
			input: `package main

//export_php:const
const TrueConstant = true

//export_php:const
const FalseConstant = false`,
			expected: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpfile, err := os.CreateTemp("", "test*.go")
			if err != nil {
				t.Fatal(err)
			}
			defer os.Remove(tmpfile.Name())

			if _, err := tmpfile.Write([]byte(tt.input)); err != nil {
				t.Fatal(err)
			}
			tmpfile.Close()

			parser := NewConstantParserWithDefRegex()
			constants, err := parser.parse(tmpfile.Name())
			if err != nil {
				t.Fatalf("parse() error = %v", err)
			}

			if len(constants) != tt.expected {
				t.Errorf("parse() got %d constants, want %d", len(constants), tt.expected)
			}

			if tt.name == "single constant" && len(constants) > 0 {
				c := constants[0]
				if c.Name != "MyConstant" {
					t.Errorf("Expected constant name 'MyConstant', got '%s'", c.Name)
				}
				if c.Value != "\"test_value\"" {
					t.Errorf("Expected constant value '\"test_value\"', got '%s'", c.Value)
				}
				if c.Type != "string" {
					t.Errorf("Expected constant type 'string', got '%s'", c.Type)
				}
				if c.IsIota {
					t.Errorf("Expected IsIota to be false for string constant")
				}
			}

			if tt.name == "iota constant" && len(constants) > 0 {
				c := constants[0]
				if c.Name != "IotaConstant" {
					t.Errorf("Expected constant name 'IotaConstant', got '%s'", c.Name)
				}
				if !c.IsIota {
					t.Errorf("Expected IsIota to be true")
				}
				if c.Value != "0" {
					t.Errorf("Expected iota constant value to be '0', got '%s'", c.Value)
				}
			}

			if tt.name == "multiple constants" && len(constants) == 3 {
				expectedNames := []string{"FirstConstant", "SecondConstant", "ThirdConstant"}
				expectedValues := []string{"\"first\"", "42", "true"}
				expectedTypes := []string{"string", "int", "bool"}

				for i, c := range constants {
					if c.Name != expectedNames[i] {
						t.Errorf("Expected constant name '%s', got '%s'", expectedNames[i], c.Name)
					}
					if c.Value != expectedValues[i] {
						t.Errorf("Expected constant value '%s', got '%s'", expectedValues[i], c.Value)
					}
					if c.Type != expectedTypes[i] {
						t.Errorf("Expected constant type '%s', got '%s'", expectedTypes[i], c.Type)
					}
				}
			}
		})
	}
}

func TestConstantParserErrors(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		expectError bool
	}{
		{
			name: "invalid constant declaration",
			input: `package main

//export_php:const
const = "missing name"`,
			expectError: true,
		},
		{
			name: "malformed constant",
			input: `package main

//export_php:const
const InvalidSyntax`,
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpfile, err := os.CreateTemp("", "test*.go")
			if err != nil {
				t.Fatal(err)
			}
			defer os.Remove(tmpfile.Name())

			if _, err := tmpfile.Write([]byte(tt.input)); err != nil {
				t.Fatal(err)
			}
			tmpfile.Close()

			parser := NewConstantParserWithDefRegex()
			_, err = parser.parse(tmpfile.Name())

			if tt.expectError && err == nil {
				t.Errorf("Expected error but got none")
			}
			if !tt.expectError && err != nil {
				t.Errorf("Unexpected error: %v", err)
			}
		})
	}
}

func TestConstantParserIotaSequence(t *testing.T) {
	input := `package main

//export_php:const
const FirstIota = iota

//export_php:const  
const SecondIota = iota

//export_php:const
const ThirdIota = iota`

	tmpfile, err := os.CreateTemp("", "test*.go")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmpfile.Name())

	if _, err := tmpfile.Write([]byte(input)); err != nil {
		t.Fatal(err)
	}
	tmpfile.Close()

	parser := NewConstantParserWithDefRegex()
	constants, err := parser.parse(tmpfile.Name())
	if err != nil {
		t.Fatalf("parse() error = %v", err)
	}

	if len(constants) != 3 {
		t.Fatalf("Expected 3 constants, got %d", len(constants))
	}

	expectedValues := []string{"0", "1", "2"}
	for i, c := range constants {
		if !c.IsIota {
			t.Errorf("Expected constant %d to be iota", i)
		}
		if c.Value != expectedValues[i] {
			t.Errorf("Expected constant %d value to be '%s', got '%s'", i, expectedValues[i], c.Value)
		}
	}
}

func TestConstantParserTypeDetection(t *testing.T) {
	tests := []struct {
		name         string
		value        string
		expectedType string
	}{
		{"string with double quotes", "\"hello world\"", "string"},
		{"string with backticks", "`hello world`", "string"},
		{"boolean true", "true", "bool"},
		{"boolean false", "false", "bool"},
		{"integer", "42", "int"},
		{"negative integer", "-42", "int"},
		{"hex integer", "0xFF", "int"},
		{"octal integer", "0755", "int"},
		{"go octal integer", "0o755", "int"},
		{"binary integer", "0b1010", "int"},
		{"float", "3.14", "float"},
		{"negative float", "-3.14", "float"},
		{"scientific notation", "1e10", "float"},
		{"unknown type", "someFunction()", "int"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := determineConstantType(tt.value)
			if result != tt.expectedType {
				t.Errorf("determineConstantType(%s) = %s, want %s", tt.value, result, tt.expectedType)
			}
		})
	}
}

func TestConstantParserRegexMatch(t *testing.T) {
	parser := NewConstantParserWithDefRegex()

	testCases := []struct {
		line     string
		expected bool
	}{
		{"//export_php:const", true},
		{"// export_php:const", true},
		{"//  export_php:const", true},
		{"//export_php:const ", false}, // should not match with trailing content
		{"//export_php", false},
		{"//export_php:function", false},
		{"//export_php:class", false},
		{"// some other comment", false},
	}

	for _, tc := range testCases {
		t.Run(tc.line, func(t *testing.T) {
			matches := parser.constRegex.MatchString(tc.line)
			if matches != tc.expected {
				t.Errorf("Expected regex match %v for line '%s', got %v", tc.expected, tc.line, matches)
			}
		})
	}
}

func TestConstantParserDeclRegex(t *testing.T) {
	parser := NewConstantParserWithDefRegex()

	testCases := []struct {
		line        string
		shouldMatch bool
		name        string
		value       string
	}{
		{"const MyConst = \"value\"", true, "MyConst", "\"value\""},
		{"const IntConst = 42", true, "IntConst", "42"},
		{"const BoolConst = true", true, "BoolConst", "true"},
		{"const IotaConst = iota", true, "IotaConst", "iota"},
		{"const ComplexValue = someFunction()", true, "ComplexValue", "someFunction()"},
		{"const SpacedName = \"with spaces\"", true, "SpacedName", "\"with spaces\""},
		{"var notAConst = \"value\"", false, "", ""},
		{"const", false, "", ""},
		{"const =", false, "", ""},
	}

	for _, tc := range testCases {
		t.Run(tc.line, func(t *testing.T) {
			matches := parser.constDeclRegex.FindStringSubmatch(tc.line)

			if tc.shouldMatch {
				if len(matches) != 3 {
					t.Errorf("Expected 3 matches for line '%s', got %d", tc.line, len(matches))
					return
				}
				if matches[1] != tc.name {
					t.Errorf("Expected name '%s', got '%s'", tc.name, matches[1])
				}
				if matches[2] != tc.value {
					t.Errorf("Expected value '%s', got '%s'", tc.value, matches[2])
				}
			} else {
				if len(matches) != 0 {
					t.Errorf("Expected no matches for line '%s', got %d", tc.line, len(matches))
				}
			}
		})
	}
}

func TestPHPConstantCValue(t *testing.T) {
	tests := []struct {
		name     string
		constant PHPConstant
		expected string
	}{
		{
			name: "octal notation 0o35",
			constant: PHPConstant{
				Name:  "OctalConst",
				Value: "0o35",
				Type:  "int",
			},
			expected: "29", // 0o35 = 29 in decimal
		},
		{
			name: "octal notation 0o755",
			constant: PHPConstant{
				Name:  "OctalPerm",
				Value: "0o755",
				Type:  "int",
			},
			expected: "493", // 0o755 = 493 in decimal
		},
		{
			name: "regular integer",
			constant: PHPConstant{
				Name:  "RegularInt",
				Value: "42",
				Type:  "int",
			},
			expected: "42",
		},
		{
			name: "hex integer",
			constant: PHPConstant{
				Name:  "HexInt",
				Value: "0xFF",
				Type:  "int",
			},
			expected: "0xFF", // hex should remain unchanged
		},
		{
			name: "string constant",
			constant: PHPConstant{
				Name:  "StringConst",
				Value: "\"hello\"",
				Type:  "string",
			},
			expected: "\"hello\"", // strings should remain unchanged
		},
		{
			name: "boolean constant",
			constant: PHPConstant{
				Name:  "BoolConst",
				Value: "true",
				Type:  "bool",
			},
			expected: "true", // booleans should remain unchanged
		},
		{
			name: "float constant",
			constant: PHPConstant{
				Name:  "FloatConst",
				Value: "3.14",
				Type:  "float",
			},
			expected: "3.14", // floats should remain unchanged
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.constant.CValue()
			if result != tt.expected {
				t.Errorf("CValue() = %s, want %s", result, tt.expected)
			}
		})
	}
}
