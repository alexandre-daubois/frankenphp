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

//export_php class User
type UserStruct struct {
	Name string
	Age  int
}`,
			expected: 1,
		},
		{
			name: "multiple classes",
			input: `package main

//export_php class User
type UserStruct struct {
	Name string
	Age  int
}

//export_php: class Product
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

//export_php: class OptionalData
type OptionalStruct struct {
	Required string
	Optional *string
	Count    *int
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

// export_php class TestClass
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

// export_php class NullableClass
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

//export_php: class CollectionClass
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
