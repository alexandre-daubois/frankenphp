package extgen

import (
	"fmt"
	"strings"
	"testing"
)

func TestPHPClassGenerator_Generate(t *testing.T) {
	tests := []struct {
		name     string
		classes  []PHPClass
		contains []string
	}{
		{
			name: "single class",
			classes: []PHPClass{
				{
					Name:     "User",
					GoStruct: "UserStruct",
					Properties: []ClassProperty{
						{Name: "id", Type: "int"},
						{Name: "name", Type: "string"},
					},
				},
			},
			contains: []string{
				"static zend_class_entry *user_ce = NULL;",
				"PHP_METHOD(User, __construct)",
				"void register_all_classes() {",
				"user_ce = register_class_User();",
				"if (!user_ce) {",
				"php_error_docref(NULL, E_ERROR, \"Failed to register class User\");",
				"return;",
				"}",
			},
		},
		{
			name: "multiple classes",
			classes: []PHPClass{
				{
					Name:     "User",
					GoStruct: "UserStruct",
					Properties: []ClassProperty{
						{Name: "id", Type: "int"},
						{Name: "name", Type: "string"},
					},
				},
				{
					Name:     "Product",
					GoStruct: "ProductStruct",
					Properties: []ClassProperty{
						{Name: "title", Type: "string"},
						{Name: "price", Type: "float"},
					},
				},
			},
			contains: []string{
				"static zend_class_entry *user_ce = NULL;",
				"static zend_class_entry *product_ce = NULL;",
				"PHP_METHOD(User, __construct)",
				"PHP_METHOD(Product, __construct)",
				"user_ce = register_class_User();",
				"product_ce = register_class_Product();",
			},
		},
		{
			name: "class with complex name",
			classes: []PHPClass{
				{
					Name:     "ComplexClassName",
					GoStruct: "ComplexStruct",
					Properties: []ClassProperty{
						{Name: "data", Type: "array"},
					},
				},
			},
			contains: []string{
				"static zend_class_entry *complexclassname_ce = NULL;",
				"PHP_METHOD(ComplexClassName, __construct)",
				"complexclassname_ce = register_class_ComplexClassName();",
				"Failed to register class ComplexClassName",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			generator := PHPClassGenerator{}
			result := generator.generate(tt.classes)

			for _, expected := range tt.contains {
				if !strings.Contains(result, expected) {
					t.Errorf("Generated class code should contain '%s'\nGenerated:\n%s", expected, result)
				}
			}
		})
	}
}

func TestPHPClassGenerator_EmptyClasses(t *testing.T) {
	generator := PHPClassGenerator{}
	result := generator.generate([]PHPClass{})

	expectedContent := "void register_all_classes() {\n    // No classes to register\n}\n\n"
	if result != expectedContent {
		t.Errorf("Expected exact content for empty classes:\nExpected:\n%s\nGot:\n%s", expectedContent, result)
	}
}

func TestPHPClassGenerator_SingleClass(t *testing.T) {
	classes := []PHPClass{
		{
			Name:     "TestClass",
			GoStruct: "TestStruct",
			Properties: []ClassProperty{
				{Name: "id", Type: "int"},
				{Name: "name", Type: "string"},
				{Name: "active", Type: "bool"},
			},
		},
	}

	generator := PHPClassGenerator{}
	result := generator.generate(classes)

	if !strings.Contains(result, "static zend_class_entry *testclass_ce = NULL;") {
		t.Error("Should contain class entry variable declaration")
	}

	expectedConstructor := `PHP_METHOD(TestClass, __construct) {
    if (zend_parse_parameters_none() == FAILURE) {
        RETURN_THROWS();
    }
}`
	if !strings.Contains(result, expectedConstructor) {
		t.Error("Should contain proper constructor method")
	}

	if !strings.Contains(result, "void register_all_classes() {") {
		t.Error("Should contain register_all_classes function")
	}

	if !strings.Contains(result, "testclass_ce = register_class_TestClass();") {
		t.Error("Should contain class registration call")
	}

	if !strings.Contains(result, "if (!testclass_ce) {") {
		t.Error("Should contain error checking for class registration")
	}

	if !strings.Contains(result, "php_error_docref(NULL, E_ERROR, \"Failed to register class TestClass\");") {
		t.Error("Should contain error message for failed registration")
	}
}

func TestPHPClassGenerator_MultipleClasses(t *testing.T) {
	classes := []PHPClass{
		{Name: "User", GoStruct: "UserStruct"},
		{Name: "Product", GoStruct: "ProductStruct"},
		{Name: "Order", GoStruct: "OrderStruct"},
	}

	generator := PHPClassGenerator{}
	result := generator.generate(classes)

	expectedVars := []string{
		"static zend_class_entry *user_ce = NULL;",
		"static zend_class_entry *product_ce = NULL;",
		"static zend_class_entry *order_ce = NULL;",
	}

	for _, expected := range expectedVars {
		if !strings.Contains(result, expected) {
			t.Errorf("Should contain class entry variable: %s", expected)
		}
	}

	expectedConstructors := []string{
		"PHP_METHOD(User, __construct)",
		"PHP_METHOD(Product, __construct)",
		"PHP_METHOD(Order, __construct)",
	}

	for _, expected := range expectedConstructors {
		if !strings.Contains(result, expected) {
			t.Errorf("Should contain constructor: %s", expected)
		}
	}

	expectedRegistrations := []string{
		"user_ce = register_class_User();",
		"product_ce = register_class_Product();",
		"order_ce = register_class_Order();",
	}

	for _, expected := range expectedRegistrations {
		if !strings.Contains(result, expected) {
			t.Errorf("Should contain registration: %s", expected)
		}
	}

	expectedErrors := []string{
		"Failed to register class User",
		"Failed to register class Product",
		"Failed to register class Order",
	}

	for _, expected := range expectedErrors {
		if !strings.Contains(result, expected) {
			t.Errorf("Should contain error message: %s", expected)
		}
	}
}

func TestPHPClassGenerator_ClassNameSanitization(t *testing.T) {
	tests := []struct {
		className   string
		expectedVar string
		expectedReg string
	}{
		{
			className:   "SimpleClass",
			expectedVar: "simpleclass_ce",
			expectedReg: "register_class_SimpleClass",
		},
		{
			className:   "ComplexClassName",
			expectedVar: "complexclassname_ce",
			expectedReg: "register_class_ComplexClassName",
		},
		{
			className:   "ClassWithNumbers123",
			expectedVar: "classwithnumbers123_ce",
			expectedReg: "register_class_ClassWithNumbers123",
		},
		{
			className:   "A",
			expectedVar: "a_ce",
			expectedReg: "register_class_A",
		},
	}

	for _, tt := range tests {
		t.Run(tt.className, func(t *testing.T) {
			classes := []PHPClass{
				{Name: tt.className, GoStruct: tt.className + "Struct"},
			}

			generator := PHPClassGenerator{}
			result := generator.generate(classes)

			expectedVarDecl := "static zend_class_entry *" + tt.expectedVar + " = NULL;"
			if !strings.Contains(result, expectedVarDecl) {
				t.Errorf("Should contain sanitized variable: %s", expectedVarDecl)
			}

			expectedRegCall := tt.expectedVar + " = " + tt.expectedReg + "();"
			if !strings.Contains(result, expectedRegCall) {
				t.Errorf("Should contain registration call: %s", expectedRegCall)
			}

			expectedErrorCheck := "if (!" + tt.expectedVar + ") {"
			if !strings.Contains(result, expectedErrorCheck) {
				t.Errorf("Should contain error check: %s", expectedErrorCheck)
			}
		})
	}
}

func TestPHPClassGenerator_ConstructorGeneration(t *testing.T) {
	classes := []PHPClass{
		{Name: "TestClass1", GoStruct: "TestStruct1"},
		{Name: "TestClass2", GoStruct: "TestStruct2"},
	}

	generator := PHPClassGenerator{}
	result := generator.generate(classes)

	for _, class := range classes {
		expectedStart := "PHP_METHOD(" + class.Name + ", __construct) {"
		expectedParseParams := "if (zend_parse_parameters_none() == FAILURE) {"
		expectedReturn := "RETURN_THROWS();"
		if !strings.Contains(result, expectedStart) {
			t.Errorf("Constructor should start with: %s", expectedStart)
		}

		if !strings.Contains(result, expectedParseParams) {
			t.Errorf("Constructor should contain parameter parsing: %s", expectedParseParams)
		}

		if !strings.Contains(result, expectedReturn) {
			t.Errorf("Constructor should contain return statement: %s", expectedReturn)
		}

		constructorStart := strings.Index(result, expectedStart)
		if constructorStart == -1 {
			continue
		}

		remaining := result[constructorStart:]
		braceCount := 0
		foundClosing := false

		for i, char := range remaining {
			if char == '{' {
				braceCount++
			} else if char == '}' {
				braceCount--
				if braceCount == 0 {
					foundClosing = true
					// Check if there's a newline after the closing brace
					if i+1 < len(remaining) && remaining[i+1] == '\n' {
						t.Logf("Constructor for %s is properly closed", class.Name)
					}
					break
				}
			}
		}

		if !foundClosing {
			t.Errorf("Constructor for %s should be properly closed", class.Name)
		}
	}
}

func TestPHPClassGenerator_RegistrationFunction(t *testing.T) {
	classes := []PHPClass{
		{Name: "Class1", GoStruct: "Struct1"},
		{Name: "Class2", GoStruct: "Struct2"},
	}

	generator := PHPClassGenerator{}
	result := generator.generate(classes)

	if !strings.Contains(result, "void register_all_classes() {") {
		t.Error("Should contain register_all_classes function signature")
	}

	if !strings.Contains(result, "}\n\n") {
		t.Error("Function should end with proper formatting")
	}

	class1Pos := strings.Index(result, "class1_ce = register_class_Class1();")
	class2Pos := strings.Index(result, "class2_ce = register_class_Class2();")

	if class1Pos == -1 {
		t.Error("Class1 registration not found")
	}

	if class2Pos == -1 {
		t.Error("Class2 registration not found")
	}

	if class1Pos >= class2Pos {
		t.Error("Classes should be registered in input order")
	}

	expectedErrorPattern := []string{
		"if (!class1_ce) {",
		"php_error_docref(NULL, E_ERROR, \"Failed to register class Class1\");",
		"return;",
		"}",
	}

	for _, pattern := range expectedErrorPattern {
		if !strings.Contains(result, pattern) {
			t.Errorf("Should contain error handling pattern: %s", pattern)
		}
	}
}

func TestPHPClassGenerator_CodeStructure(t *testing.T) {
	classes := []PHPClass{
		{Name: "TestClass", GoStruct: "TestStruct"},
	}

	generator := PHPClassGenerator{}
	result := generator.generate(classes)

	lines := strings.Split(result, "\n")
	sections := []string{
		"static zend_class_entry",
		"PHP_METHOD",
		"void register_all_classes",
	}

	lastFoundIndex := -1
	for _, section := range sections {
		sectionFound := false
		for i, line := range lines {
			if strings.Contains(line, section) && i > lastFoundIndex {
				lastFoundIndex = i
				sectionFound = true
				break
			}
		}
		if !sectionFound {
			t.Errorf("Section '%s' not found in correct order", section)
		}
	}

	if strings.Contains(result, "  ") {
		properlyIndentedLines := 0
		for _, line := range lines {
			if strings.HasPrefix(line, "    ") || strings.TrimSpace(line) == line {
				properlyIndentedLines++
			}
		}
	}

	openBraces := strings.Count(result, "{")
	closeBraces := strings.Count(result, "}")
	if openBraces != closeBraces {
		t.Errorf("Unbalanced braces: %d open, %d close", openBraces, closeBraces)
	}

	lines = strings.Split(result, "\n")
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "//") {
			continue
		}

		if (strings.Contains(trimmed, "php_error_docref") ||
			strings.Contains(trimmed, "return;") ||
			strings.Contains(trimmed, "_ce = register_class_")) &&
			!strings.HasSuffix(trimmed, ";") {
			t.Errorf("Line %d should end with semicolon: %s", i+1, trimmed)
		}
	}
}

func TestPHPClassGenerator_ErrorHandling(t *testing.T) {
	classes := []PHPClass{
		{Name: "ErrorTestClass", GoStruct: "ErrorTestStruct"},
	}

	generator := PHPClassGenerator{}
	result := generator.generate(classes)

	errorComponents := []string{
		"if (!errortestclass_ce) {",
		"php_error_docref(NULL, E_ERROR,",
		"\"Failed to register class ErrorTestClass\"",
		"return;",
	}

	for _, component := range errorComponents {
		if !strings.Contains(result, component) {
			t.Errorf("Error handling should contain: %s", component)
		}
	}

	if !strings.Contains(result, "}\n}") {
		t.Error("Error handling should be properly nested within registration function")
	}
}

func TestPHPClassGenerator_EdgeCases(t *testing.T) {
	tests := []struct {
		name        string
		classes     []PHPClass
		expectPanic bool
	}{
		{
			name:    "empty class name",
			classes: []PHPClass{{Name: "", GoStruct: "EmptyStruct"}},
		},
		{
			name:    "class with empty go struct",
			classes: []PHPClass{{Name: "ValidClass", GoStruct: ""}},
		},
		{
			name: "class with no properties",
			classes: []PHPClass{{
				Name:       "NoProps",
				GoStruct:   "NoPropsStruct",
				Properties: []ClassProperty{},
			}},
		},
		{
			name: "class with many properties",
			classes: []PHPClass{{
				Name:     "ManyProps",
				GoStruct: "ManyPropsStruct",
				Properties: []ClassProperty{
					{Name: "prop1", Type: "string"},
					{Name: "prop2", Type: "int"},
					{Name: "prop3", Type: "float"},
					{Name: "prop4", Type: "bool"},
					{Name: "prop5", Type: "array"},
					{Name: "prop6", Type: "object"},
				},
			}},
		},
	}

	generator := PHPClassGenerator{}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			defer func() {
				if r := recover(); r != nil {
					if !tt.expectPanic {
						t.Errorf("Unexpected panic: %v", r)
					}
				}
			}()

			result := generator.generate(tt.classes)

			// Basic sanity checks for any generated result
			if len(tt.classes) == 0 {
				if !strings.Contains(result, "// No classes to register") {
					t.Error("Empty classes should generate no-op message")
				}
			} else {
				if !strings.Contains(result, "void register_all_classes()") {
					t.Error("Should always contain registration function")
				}
			}
		})
	}
}

func TestPHPClassGenerator_PropertyHandling(t *testing.T) {
	classes := []PHPClass{
		{
			Name:     "PropClass",
			GoStruct: "PropStruct",
			Properties: []ClassProperty{
				{Name: "id", Type: "int", IsNullable: false},
				{Name: "name", Type: "string", IsNullable: false},
				{Name: "email", Type: "string", IsNullable: true},
				{Name: "settings", Type: "array", IsNullable: true},
				{Name: "profile", Type: "object", IsNullable: true},
			},
		},
	}

	generator := PHPClassGenerator{}
	result := generator.generate(classes)

	expectedElements := []string{
		"static zend_class_entry *propclass_ce = NULL;",
		"PHP_METHOD(PropClass, __construct)",
		"propclass_ce = register_class_PropClass();",
	}

	for _, element := range expectedElements {
		if !strings.Contains(result, element) {
			t.Errorf("Should contain element even with properties: %s", element)
		}
	}
}

func TestPHPClassGenerator_Performance(t *testing.T) {
	var classes []PHPClass
	for i := 0; i < 100; i++ {
		classes = append(classes, PHPClass{
			Name:     fmt.Sprintf("Class%d", i),
			GoStruct: fmt.Sprintf("Struct%d", i),
			Properties: []ClassProperty{
				{Name: "id", Type: "int"},
				{Name: "data", Type: "string"},
			},
		})
	}

	generator := PHPClassGenerator{}
	result := generator.generate(classes)

	for i := 0; i < 100; i++ {
		expectedClass := fmt.Sprintf("Class%d", i)
		if !strings.Contains(result, expectedClass) {
			t.Errorf("Missing class in large generation: %s", expectedClass)
		}
	}

	if len(result) < 1000 {
		t.Error("Generated code seems too small for 100 classes")
	}

	if !strings.Contains(result, "void register_all_classes() {") {
		t.Error("Large generation should maintain proper structure")
	}
}

func TestPHPClassGenerator_SpecialCharacters(t *testing.T) {
	tests := []struct {
		name      string
		className string
	}{
		{"unicode class", "UserClass"},
		{"numbers", "User123"},
		{"single char", "U"},
		{"long name", "VeryLongClassNameThatShouldStillWork"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			classes := []PHPClass{
				{Name: tt.className, GoStruct: tt.className + "Struct"},
			}

			generator := PHPClassGenerator{}
			result := generator.generate(classes)

			expectedConstructor := "PHP_METHOD(" + tt.className + ", __construct)"
			if !strings.Contains(result, expectedConstructor) {
				t.Errorf("Should contain constructor for class: %s", tt.className)
			}

			expectedReg := "register_class_" + tt.className + "();"
			if !strings.Contains(result, expectedReg) {
				t.Errorf("Should contain registration call: %s", expectedReg)
			}
		})
	}
}

func TestPHPClassGenerator_ConsistentOutput(t *testing.T) {
	classes := []PHPClass{
		{Name: "ConsistentClass", GoStruct: "ConsistentStruct"},
	}

	generator := PHPClassGenerator{}
	result1 := generator.generate(classes)
	result2 := generator.generate(classes)

	if result1 != result2 {
		t.Error("Generator should produce consistent output for same input")
	}

	classes2 := []PHPClass{
		{Name: "ClassB", GoStruct: "StructB"},
		{Name: "ClassA", GoStruct: "StructA"},
	}
	classes3 := []PHPClass{
		{Name: "ClassA", GoStruct: "StructA"},
		{Name: "ClassB", GoStruct: "StructB"},
	}

	result3 := generator.generate(classes2)
	result4 := generator.generate(classes3)

	if result3 == result4 {
		t.Error("Generator should respect input order")
	}

	for _, result := range []string{result3, result4} {
		if !strings.Contains(result, "ClassA") || !strings.Contains(result, "ClassB") {
			t.Error("Both results should contain both classes")
		}
	}
}
