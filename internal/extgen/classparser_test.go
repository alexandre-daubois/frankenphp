package extgen

import (
	"os"
	"testing"
)

func TestClassParser(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected int
	}{
		{
			name: "single class",
			input: `package main

//export_php:class User
type UserStruct struct {
	Name string
	Age  int
}`,
			expected: 1,
		},
		{
			name: "multiple classes",
			input: `package main

//export_php:class User
type UserStruct struct {
	Name string
	Age  int
}

//export_php:class Product
type ProductStruct struct {
	Title string
	Price float64
}`,
			expected: 2,
		},
		{
			name: "no php classes",
			input: `package main

type RegularStruct struct {
	Data string
}`,
			expected: 0,
		},
		{
			name: "class with nullable fields",
			input: `package main

//export_php:class OptionalData
type OptionalStruct struct {
	Required string
	Optional *string
	Count    *int
}`,
			expected: 1,
		},
		{
			name: "class with methods",
			input: `package main

//export_php:class User
type UserStruct struct {
	Name string
	Age  int
}

//export_php:method User::getName(): string
func GetUserName(u UserStruct) string {
	return u.Name
}

//export_php:method User::setAge(int $age): void
func SetUserAge(u *UserStruct, age int) {
	u.Age = age
}`,
			expected: 1,
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

			parser := ClassParser{}
			classes, err := parser.parse(tmpfile.Name())
			if err != nil {
				t.Fatalf("parse() error = %v", err)
			}

			if len(classes) != tt.expected {
				t.Errorf("parse() got %d classes, want %d", len(classes), tt.expected)
			}

			if tt.name == "single class" && len(classes) > 0 {
				class := classes[0]
				if class.Name != "User" {
					t.Errorf("Expected class name 'User', got '%s'", class.Name)
				}
				if class.GoStruct != "UserStruct" {
					t.Errorf("Expected Go struct 'UserStruct', got '%s'", class.GoStruct)
				}
				if len(class.Properties) != 2 {
					t.Errorf("Expected 2 properties, got %d", len(class.Properties))
				}
			}

			if tt.name == "class with nullable fields" && len(classes) > 0 {
				class := classes[0]
				if len(class.Properties) >= 3 {
					if class.Properties[0].IsNullable {
						t.Errorf("Required field should not be nullable")
					}
					if !class.Properties[1].IsNullable {
						t.Errorf("Optional field should be nullable")
					}
					if !class.Properties[2].IsNullable {
						t.Errorf("Count field should be nullable")
					}
				}
			}
		})
	}
}

func TestClassMethods(t *testing.T) {
	input := `package main

//export_php:class User
type UserStruct struct {
	Name string
	Age  int
}

//export_php:method User::getName(): string
func GetUserName(u UserStruct) string {
	return u.Name
}

//export_php:method User::setAge(int $age): void
func SetUserAge(u *UserStruct, age int) {
	u.Age = age
}

//export_php:method User::getInfo(string $prefix = "User"): string
func GetUserInfo(u UserStruct, prefix string) string {
	return prefix + ": " + u.Name
}`

	tmpfile, err := os.CreateTemp("", "test*.go")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmpfile.Name())

	if _, err := tmpfile.Write([]byte(input)); err != nil {
		t.Fatal(err)
	}
	tmpfile.Close()

	parser := ClassParser{}
	classes, err := parser.parse(tmpfile.Name())
	if err != nil {
		t.Fatalf("parse() error = %v", err)
	}

	if len(classes) != 1 {
		t.Fatalf("Expected 1 class, got %d", len(classes))
	}

	class := classes[0]
	if len(class.Methods) != 3 {
		t.Fatalf("Expected 3 methods, got %d", len(class.Methods))
	}

	getName := class.Methods[0]
	if getName.Name != "getName" {
		t.Errorf("Expected method name 'getName', got '%s'", getName.Name)
	}
	if getName.ReturnType != "string" {
		t.Errorf("Expected return type 'string', got '%s'", getName.ReturnType)
	}
	if len(getName.Params) != 0 {
		t.Errorf("Expected 0 params, got %d", len(getName.Params))
	}
	if getName.ClassName != "User" {
		t.Errorf("Expected class name 'User', got '%s'", getName.ClassName)
	}

	setAge := class.Methods[1]
	if setAge.Name != "setAge" {
		t.Errorf("Expected method name 'setAge', got '%s'", setAge.Name)
	}
	if setAge.ReturnType != "void" {
		t.Errorf("Expected return type 'void', got '%s'", setAge.ReturnType)
	}
	if len(setAge.Params) != 1 {
		t.Errorf("Expected 1 param, got %d", len(setAge.Params))
	}
	if len(setAge.Params) > 0 {
		param := setAge.Params[0]
		if param.Name != "age" {
			t.Errorf("Expected param name 'age', got '%s'", param.Name)
		}
		if param.Type != "int" {
			t.Errorf("Expected param type 'int', got '%s'", param.Type)
		}
		if param.IsNullable {
			t.Errorf("Expected param to not be nullable")
		}
		if param.HasDefault {
			t.Errorf("Expected param to not have default value")
		}
	}

	getInfo := class.Methods[2]
	if getInfo.Name != "getInfo" {
		t.Errorf("Expected method name 'getInfo', got '%s'", getInfo.Name)
	}
	if getInfo.ReturnType != "string" {
		t.Errorf("Expected return type 'string', got '%s'", getInfo.ReturnType)
	}
	if len(getInfo.Params) != 1 {
		t.Errorf("Expected 1 param, got %d", len(getInfo.Params))
	}
	if len(getInfo.Params) > 0 {
		param := getInfo.Params[0]
		if param.Name != "prefix" {
			t.Errorf("Expected param name 'prefix', got '%s'", param.Name)
		}
		if param.Type != "string" {
			t.Errorf("Expected param type 'string', got '%s'", param.Type)
		}
		if !param.HasDefault {
			t.Errorf("Expected param to have default value")
		}
		if param.DefaultValue != "User" {
			t.Errorf("Expected default value 'User', got '%s'", param.DefaultValue)
		}
	}
}

func TestMethodParameterParsing(t *testing.T) {
	tests := []struct {
		name          string
		paramStr      string
		expectedParam Parameter
		expectError   bool
	}{
		{
			name:     "simple int parameter",
			paramStr: "int $age",
			expectedParam: Parameter{
				Name:       "age",
				Type:       "int",
				IsNullable: false,
				HasDefault: false,
			},
			expectError: false,
		},
		{
			name:     "nullable string parameter",
			paramStr: "?string $name",
			expectedParam: Parameter{
				Name:       "name",
				Type:       "string",
				IsNullable: true,
				HasDefault: false,
			},
			expectError: false,
		},
		{
			name:     "parameter with default value",
			paramStr: "string $prefix = \"default\"",
			expectedParam: Parameter{
				Name:         "prefix",
				Type:         "string",
				IsNullable:   false,
				HasDefault:   true,
				DefaultValue: "default",
			},
			expectError: false,
		},
		{
			name:     "nullable parameter with default null",
			paramStr: "?int $count = null",
			expectedParam: Parameter{
				Name:         "count",
				Type:         "int",
				IsNullable:   true,
				HasDefault:   true,
				DefaultValue: "null",
			},
			expectError: false,
		},
		{
			name:        "invalid parameter format",
			paramStr:    "invalid",
			expectError: true,
		},
	}

	parser := ClassParser{}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			param, err := parser.parseMethodParameter(tt.paramStr)

			if tt.expectError {
				if err == nil {
					t.Errorf("Expected error for parameter '%s', but got none", tt.paramStr)
				}
				return
			}

			if err != nil {
				t.Errorf("parseMethodParameter(%s) error = %v", tt.paramStr, err)
				return
			}

			if param.Name != tt.expectedParam.Name {
				t.Errorf("Expected name '%s', got '%s'", tt.expectedParam.Name, param.Name)
			}
			if param.Type != tt.expectedParam.Type {
				t.Errorf("Expected type '%s', got '%s'", tt.expectedParam.Type, param.Type)
			}
			if param.IsNullable != tt.expectedParam.IsNullable {
				t.Errorf("Expected IsNullable %v, got %v", tt.expectedParam.IsNullable, param.IsNullable)
			}
			if param.HasDefault != tt.expectedParam.HasDefault {
				t.Errorf("Expected HasDefault %v, got %v", tt.expectedParam.HasDefault, param.HasDefault)
			}
			if param.DefaultValue != tt.expectedParam.DefaultValue {
				t.Errorf("Expected DefaultValue '%s', got '%s'", tt.expectedParam.DefaultValue, param.DefaultValue)
			}
		})
	}
}

func TestGoTypeToPHPType(t *testing.T) {
	tests := []struct {
		goType   string
		expected string
	}{
		{"string", "string"},
		{"*string", "string"},
		{"int", "int"},
		{"int64", "int"},
		{"*int", "int"},
		{"float64", "float"},
		{"*float32", "float"},
		{"bool", "bool"},
		{"*bool", "bool"},
		{"[]string", "array"},
		{"map[string]int", "array"},
		{"*[]int", "array"},
		{"interface{}", "mixed"},
		{"CustomType", "mixed"},
	}

	parser := ClassParser{}
	for _, tt := range tests {
		t.Run(tt.goType, func(t *testing.T) {
			result := parser.goTypeToPHPType(tt.goType)
			if result != tt.expected {
				t.Errorf("goTypeToPHPType(%s) = %s, want %s", tt.goType, result, tt.expected)
			}
		})
	}
}

func TestTypeToString(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected []string
	}{
		{
			name: "basic types",
			input: `package main

//export_php:class TestClass
type TestStruct struct {
	StringField string
	IntField    int
	FloatField  float64
	BoolField   bool
}`,
			expected: []string{"string", "int", "float", "bool"},
		},
		{
			name: "pointer types",
			input: `package main

//export_php:class NullableClass
type NullableStruct struct {
	NullableString *string
	NullableInt    *int
	NullableFloat  *float64
	NullableBool   *bool
}`,
			expected: []string{"string", "int", "float", "bool"},
		},
		{
			name: "collection types",
			input: `package main

//export_php:class CollectionClass
type CollectionStruct struct {
	StringSlice []string
	IntMap      map[string]int
	MixedSlice  []interface{}
}`,
			expected: []string{"array", "array", "array"},
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

			parser := ClassParser{}
			classes, err := parser.parse(tmpfile.Name())
			if err != nil {
				t.Fatalf("parse() error = %v", err)
			}

			if len(classes) != 1 {
				t.Fatalf("Expected 1 class, got %d", len(classes))
			}

			class := classes[0]
			if len(class.Properties) != len(tt.expected) {
				t.Fatalf("Expected %d properties, got %d", len(tt.expected), len(class.Properties))
			}

			for i, expectedType := range tt.expected {
				if class.Properties[i].Type != expectedType {
					t.Errorf("Property %d: expected type %s, got %s",
						i, expectedType, class.Properties[i].Type)
				}
			}
		})
	}
}
