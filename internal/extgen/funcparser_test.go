package extgen

import (
	"os"
	"testing"
)

func TestFunctionParser(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected int
	}{
		{
			name: "single function",
			input: `package main

//export_php function testFunc(string $name): string
func testFunc(name *go_string) *go_value {
	return String("Hello " + CStringToGoString(name))
}`,
			expected: 1,
		},
		{
			name: "multiple functions",
			input: `package main

//export_php function func1(int $a): int
func func1(a long) *go_value {
	return Int(a * 2)
}

//export_php function func2(string $b): string  
func func2(b *go_string) *go_value {
	return String("processed: " + CStringToGoString(b))
}`,
			expected: 2,
		},
		{
			name: "no php functions",
			input: `package main

func regularFunc() {
	// Just a regular Go function
}`,
			expected: 0,
		},
		{
			name: "mixed functions",
			input: `package main

//export_php function phpFunc(string $data): string
func phpFunc(data *go_string) *go_value {
	return String("PHP: " + CStringToGoString(data))
}

func internalFunc() {
	// Internal function without export_php comment
}

//export_php function anotherPhpFunc(int $num): int
func anotherPhpFunc(num long) *go_value {
	return Int(num * 10)
}`,
			expected: 2,
		},
		{
			name: "wrong args syntax",
			input: `package main

//export_php function phpFunc(data string): string
func phpFunc(data *go_string) *go_value {
	return String("PHP: " + CStringToGoString(data))
}`,
			expected: 0,
		},
		{
			name: "decoupled function names",
			input: `package main

//export_php function my_php_function(string $name): string
func myGoFunction(name *go_string) *go_value {
	return String("Hello " + CStringToGoString(name))
}

//export_php function another_php_func(int $num): int
func someOtherGoName(num long) *go_value {
	return Int(num * 5)
}`,
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

			parser := NewFuncParserDefRegex()
			functions, err := parser.parse(tmpfile.Name())
			if err != nil {
				t.Fatalf("parse() error = %v", err)
			}

			if len(functions) != tt.expected {
				t.Errorf("parse() got %d functions, want %d", len(functions), tt.expected)
			}

			if tt.name == "single function" && len(functions) > 0 {
				fn := functions[0]
				if fn.Name != "testFunc" {
					t.Errorf("Expected function name 'testFunc', got '%s'", fn.Name)
				}
				if fn.ReturnType != "string" {
					t.Errorf("Expected return type 'string', got '%s'", fn.ReturnType)
				}
				if len(fn.Params) != 1 {
					t.Errorf("Expected 1 parameter, got %d", len(fn.Params))
				}
				if len(fn.Params) > 0 && fn.Params[0].Name != "name" {
					t.Errorf("Expected parameter name 'name', got '%s'", fn.Params[0].Name)
				}
			}

			if tt.name == "decoupled function names" && len(functions) >= 2 {
				fn1 := functions[0]
				if fn1.Name != "my_php_function" {
					t.Errorf("Expected PHP function name 'my_php_function', got '%s'", fn1.Name)
				}
				fn2 := functions[1]
				if fn2.Name != "another_php_func" {
					t.Errorf("Expected PHP function name 'another_php_func', got '%s'", fn2.Name)
				}
			}
		})
	}
}

func TestSignatureParsing(t *testing.T) {
	tests := []struct {
		name        string
		signature   string
		expectError bool
		funcName    string
		paramCount  int
		returnType  string
		nullable    bool
	}{
		{
			name:       "simple function",
			signature:  "test(name string): string",
			funcName:   "test",
			paramCount: 1,
			returnType: "string",
			nullable:   false,
		},
		{
			name:       "nullable return",
			signature:  "test(id int): ?string",
			funcName:   "test",
			paramCount: 1,
			returnType: "string",
			nullable:   true,
		},
		{
			name:       "multiple params",
			signature:  "calculate(a int, b float, name string): float",
			funcName:   "calculate",
			paramCount: 3,
			returnType: "float",
			nullable:   false,
		},
		{
			name:       "no parameters",
			signature:  "getValue(): int",
			funcName:   "getValue",
			paramCount: 0,
			returnType: "int",
			nullable:   false,
		},
		{
			name:       "nullable parameters",
			signature:  "process(?string data, ?int count): bool",
			funcName:   "process",
			paramCount: 2,
			returnType: "bool",
			nullable:   false,
		},
		{
			name:        "invalid signature",
			signature:   "invalid syntax here",
			expectError: true,
		},
		{
			name:        "missing return type",
			signature:   "test(name string)",
			expectError: true,
		},
	}

	parser := NewFuncParserDefRegex()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fn, err := parser.parseSignature(tt.signature)

			if tt.expectError {
				if err == nil {
					t.Errorf("parseSignature() expected error but got none")
				}
				return
			}

			if err != nil {
				t.Fatalf("parseSignature() error = %v", err)
			}

			if fn.Name != tt.funcName {
				t.Errorf("parseSignature() name = %v, want %v", fn.Name, tt.funcName)
			}

			if len(fn.Params) != tt.paramCount {
				t.Errorf("parseSignature() param count = %v, want %v", len(fn.Params), tt.paramCount)
			}

			if fn.ReturnType != tt.returnType {
				t.Errorf("parseSignature() return type = %v, want %v", fn.ReturnType, tt.returnType)
			}

			if fn.IsReturnNullable != tt.nullable {
				t.Errorf("parseSignature() nullable = %v, want %v", fn.IsReturnNullable, tt.nullable)
			}

			if tt.name == "nullable parameters" {
				if len(fn.Params) >= 2 {
					if !fn.Params[0].IsNullable {
						t.Errorf("First parameter should be nullable")
					}
					if !fn.Params[1].IsNullable {
						t.Errorf("Second parameter should be nullable")
					}
				}
			}
		})
	}
}

func TestParameterParsing(t *testing.T) {
	tests := []struct {
		name             string
		paramStr         string
		expectedName     string
		expectedType     string
		expectedNullable bool
		expectedDefault  string
		hasDefault       bool
		expectError      bool
	}{
		{
			name:         "simple string param",
			paramStr:     "string name",
			expectedName: "name",
			expectedType: "string",
		},
		{
			name:             "nullable int param",
			paramStr:         "?int count",
			expectedName:     "count",
			expectedType:     "int",
			expectedNullable: true,
		},
		{
			name:            "param with default",
			paramStr:        "string message = 'hello'",
			expectedName:    "message",
			expectedType:    "string",
			expectedDefault: "hello",
			hasDefault:      true,
		},
		{
			name:            "int with default",
			paramStr:        "int limit = 10",
			expectedName:    "limit",
			expectedType:    "int",
			expectedDefault: "10",
			hasDefault:      true,
		},
		{
			name:             "nullable with default",
			paramStr:         "?string data = null",
			expectedName:     "data",
			expectedType:     "string",
			expectedNullable: true,
			expectedDefault:  "null",
			hasDefault:       true,
		},
		{
			name:        "invalid format",
			paramStr:    "invalid",
			expectError: true,
		},
	}

	parser := NewFuncParserDefRegex()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			param, err := parser.parseParameter(tt.paramStr)

			if tt.expectError {
				if err == nil {
					t.Errorf("parseParameter() expected error but got none")
				}
				return
			}

			if err != nil {
				t.Fatalf("parseParameter() error = %v", err)
			}

			if param.Name != tt.expectedName {
				t.Errorf("parseParameter() name = %v, want %v", param.Name, tt.expectedName)
			}

			if param.Type != tt.expectedType {
				t.Errorf("parseParameter() type = %v, want %v", param.Type, tt.expectedType)
			}

			if param.IsNullable != tt.expectedNullable {
				t.Errorf("parseParameter() nullable = %v, want %v", param.IsNullable, tt.expectedNullable)
			}

			if param.HasDefault != tt.hasDefault {
				t.Errorf("parseParameter() hasDefault = %v, want %v", param.HasDefault, tt.hasDefault)
			}

			if tt.hasDefault && param.DefaultValue != tt.expectedDefault {
				t.Errorf("parseParameter() defaultValue = %v, want %v", param.DefaultValue, tt.expectedDefault)
			}
		})
	}
}
