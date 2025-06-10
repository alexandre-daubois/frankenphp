package extgen

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestClassMethodParsing(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.go")

	testContent := `package main

// export_php:class MySuperClass
type MyClass struct {
    Version string
}

// export_php:method MySuperClass::setVersion(string $version): void
func (mc *MyClass) SetVersion(version string) {
    mc.Version = version
}

// export_php:method MySuperClass::getVersion(): string
func (mc *MyClass) GetVersion() string {
    return mc.Version
}

// export_php:method MySuperClass::processData(string $data, int $count): string
func (mc *MyClass) ProcessData(data string, count int) string {
    result := ""
    for i := 0; i < count; i++ {
        result += data
    }
    return result
}
`

	err := os.WriteFile(testFile, []byte(testContent), 0644)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Parse the file
	parser := &ClassParser{}
	classes, err := parser.parse(testFile)
	if err != nil {
		t.Fatalf("Failed to parse file: %v", err)
	}

	// Verify we got one class
	if len(classes) != 1 {
		t.Fatalf("Expected 1 class, got %d", len(classes))
	}

	class := classes[0]
	if class.Name != "MySuperClass" {
		t.Errorf("Expected class name 'MySuperClass', got '%s'", class.Name)
	}

	if class.GoStruct != "MyClass" {
		t.Errorf("Expected Go struct name 'MyClass', got '%s'", class.GoStruct)
	}

	// Verify we got 3 methods
	if len(class.Methods) != 3 {
		t.Fatalf("Expected 3 methods, got %d", len(class.Methods))
	}

	// Test setVersion method
	setVersionMethod := class.Methods[0]
	if setVersionMethod.Name != "setVersion" {
		t.Errorf("Expected method name 'setVersion', got '%s'", setVersionMethod.Name)
	}
	if setVersionMethod.ClassName != "MySuperClass" {
		t.Errorf("Expected class name 'MySuperClass', got '%s'", setVersionMethod.ClassName)
	}
	if setVersionMethod.ReturnType != "void" {
		t.Errorf("Expected return type 'void', got '%s'", setVersionMethod.ReturnType)
	}
	if len(setVersionMethod.Params) != 1 {
		t.Fatalf("Expected 1 parameter, got %d", len(setVersionMethod.Params))
	}
	if setVersionMethod.Params[0].Name != "version" {
		t.Errorf("Expected parameter name 'version', got '%s'", setVersionMethod.Params[0].Name)
	}
	if setVersionMethod.Params[0].Type != "string" {
		t.Errorf("Expected parameter type 'string', got '%s'", setVersionMethod.Params[0].Type)
	}

	// Test getVersion method
	getVersionMethod := class.Methods[1]
	if getVersionMethod.Name != "getVersion" {
		t.Errorf("Expected method name 'getVersion', got '%s'", getVersionMethod.Name)
	}
	if getVersionMethod.ReturnType != "string" {
		t.Errorf("Expected return type 'string', got '%s'", getVersionMethod.ReturnType)
	}
	if len(getVersionMethod.Params) != 0 {
		t.Errorf("Expected 0 parameters, got %d", len(getVersionMethod.Params))
	}

	processDataMethod := class.Methods[2]
	if processDataMethod.Name != "processData" {
		t.Errorf("Expected method name 'processData', got '%s'", processDataMethod.Name)
	}
	if processDataMethod.ReturnType != "string" {
		t.Errorf("Expected return type 'string', got '%s'", processDataMethod.ReturnType)
	}
	if len(processDataMethod.Params) != 2 {
		t.Fatalf("Expected 2 parameters, got %d", len(processDataMethod.Params))
	}
	if processDataMethod.Params[0].Type != "string" {
		t.Errorf("Expected first parameter type 'string', got '%s'", processDataMethod.Params[0].Type)
	}
	if processDataMethod.Params[1].Type != "int" {
		t.Errorf("Expected second parameter type 'int', got '%s'", processDataMethod.Params[1].Type)
	}
}

func TestGoFileGenerationWithMethods(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.go")

	testContent := `package main

// export_php:class MySuperClass
type MyClass struct {
    Version string
}

// export_php:method MySuperClass::setVersion(string $version): void
func (mc *MyClass) SetVersion(version string) {
    mc.Version = version
}
`

	err := os.WriteFile(testFile, []byte(testContent), 0644)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	generator := &Generator{
		BaseName:   "myext",
		SourceFile: testFile,
		BuildDir:   tmpDir,
		Classes: []PHPClass{
			{
				Name:     "MySuperClass",
				GoStruct: "MyClass",
				Properties: []ClassProperty{
					{Name: "Version", Type: "string", GoType: "string"},
				},
				Methods: []ClassMethod{
					{
						Name:      "setVersion",
						PHPName:   "setVersion",
						ClassName: "MySuperClass",
						Params: []Parameter{
							{Name: "version", Type: "string"},
						},
						ReturnType: "void",
					},
					{
						Name:       "getVersion",
						PHPName:    "getVersion",
						ClassName:  "MySuperClass",
						Params:     []Parameter{},
						ReturnType: "string",
					},
				},
			},
		},
	}

	goGen := &GoFileGenerator{generator: generator}
	content, err := goGen.buildContent()
	if err != nil {
		t.Fatalf("Failed to build Go content: %v", err)
	}

	expectedElements := []string{
		"type MyClass struct {",
		"Version string",
		"var objectRegistry = make(map[uint64]interface{})",
		"//export registerGoObject",
		"//export getGoObject",
		"//export removeGoObject",
		"//export create_MyClass_object",
		"func create_MyClass_object() uint64",
		"obj := &MyClass{}",
		"return registerGoObject(unsafe.Pointer(obj))",
		"//export get_MySuperClass_Version_property",
		"func get_MySuperClass_Version_property(objectID uint64) string",
		"//export set_MySuperClass_Version_property",
		"func set_MySuperClass_Version_property(objectID uint64, value string)",
		"//export setVersion_wrapper",
		"func setVersion_wrapper(objectID uint64, version *C.zend_string)",
		"objPtr := getGoObject(objectID)",
		"obj := (*MyClass)(objPtr)",
		"obj.SetVersion(C.GoStringN(C.ZSTR_VAL(version), C.int(C.ZSTR_LEN(version))))",
		"//export getVersion_wrapper",
		"func getVersion_wrapper(objectID uint64) string",
		"return obj.GetVersion()",
	}

	for _, element := range expectedElements {
		if !strings.Contains(content, element) {
			t.Errorf("Expected content to contain '%s', but it didn't", element)
		}
	}
}

func TestMethodSignatureParsing(t *testing.T) {
	parser := &ClassParser{}

	tests := []struct {
		className string
		signature string
		expected  ClassMethod
	}{
		{
			className: "TestClass",
			signature: "setData(string $data): void",
			expected: ClassMethod{
				Name:             "setData",
				PHPName:          "setData",
				ClassName:        "TestClass",
				ReturnType:       "void",
				IsReturnNullable: false,
				Params: []Parameter{
					{Name: "data", Type: "string", IsNullable: false},
				},
			},
		},
		{
			className: "TestClass",
			signature: "getData(): ?string",
			expected: ClassMethod{
				Name:             "getData",
				PHPName:          "getData",
				ClassName:        "TestClass",
				ReturnType:       "string",
				IsReturnNullable: true,
				Params:           []Parameter{},
			},
		},
		{
			className: "TestClass",
			signature: "process(?string $input, int $count): array",
			expected: ClassMethod{
				Name:             "process",
				PHPName:          "process",
				ClassName:        "TestClass",
				ReturnType:       "array",
				IsReturnNullable: false,
				Params: []Parameter{
					{Name: "input", Type: "string", IsNullable: true},
					{Name: "count", Type: "int", IsNullable: false},
				},
			},
		},
	}

	for _, test := range tests {
		result, err := parser.parseMethodSignature(test.className, test.signature)
		if err != nil {
			t.Errorf("Failed to parse signature '%s': %v", test.signature, err)
			continue
		}

		if result.Name != test.expected.Name {
			t.Errorf("Expected method name '%s', got '%s'", test.expected.Name, result.Name)
		}
		if result.ClassName != test.expected.ClassName {
			t.Errorf("Expected class name '%s', got '%s'", test.expected.ClassName, result.ClassName)
		}
		if result.ReturnType != test.expected.ReturnType {
			t.Errorf("Expected return type '%s', got '%s'", test.expected.ReturnType, result.ReturnType)
		}
		if result.IsReturnNullable != test.expected.IsReturnNullable {
			t.Errorf("Expected IsReturnNullable %v, got %v", test.expected.IsReturnNullable, result.IsReturnNullable)
		}
		if len(result.Params) != len(test.expected.Params) {
			t.Errorf("Expected %d parameters, got %d", len(test.expected.Params), len(result.Params))
			continue
		}

		for i, param := range result.Params {
			expectedParam := test.expected.Params[i]
			if param.Name != expectedParam.Name {
				t.Errorf("Expected parameter name '%s', got '%s'", expectedParam.Name, param.Name)
			}
			if param.Type != expectedParam.Type {
				t.Errorf("Expected parameter type '%s', got '%s'", expectedParam.Type, param.Type)
			}
			if param.IsNullable != expectedParam.IsNullable {
				t.Errorf("Expected parameter IsNullable %v, got %v", expectedParam.IsNullable, param.IsNullable)
			}
		}
	}
}
