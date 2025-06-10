package extgen

import (
	"fmt"
	"path/filepath"
	"strings"
)

type GoFileGenerator struct {
	generator *Generator
}

func (gg *GoFileGenerator) generate() error {
	filename := filepath.Join(gg.generator.BuildDir, gg.generator.BaseName+".go")
	content, err := gg.buildContent()
	if err != nil {
		return fmt.Errorf("building Go file content: %w", err)
	}
	return WriteFile(filename, content)
}

func (gg *GoFileGenerator) buildContent() (string, error) {
	sourceAnalyzer := SourceAnalyzer{}
	imports, internalFunctions, err := sourceAnalyzer.Analyze(gg.generator.SourceFile)
	if err != nil {
		return "", fmt.Errorf("analyzing source file: %w", err)
	}

	var builder strings.Builder

	cleanPackageName := SanitizePackageName(gg.generator.BaseName)
	builder.WriteString(fmt.Sprintf(`package %s

/*
#include <stdlib.h>
#include "%s.h"
*/
import "C"
import (
	"sync"
	"unsafe"
)
`, cleanPackageName, gg.generator.BaseName))

	for _, imp := range imports {
		if imp == `"C"` {
			continue
		}

		builder.WriteString(fmt.Sprintf("import %s\n", imp))
	}

	builder.WriteString("\nfunc init() {\n\tC.register_extension()\n}\n\n") // TODO update with the new frankenphp func!

	for _, constant := range gg.generator.Constants {
		builder.WriteString(fmt.Sprintf("const %s = %s\n", constant.Name, constant.Value))
	}

	if len(gg.generator.Constants) > 0 {
		builder.WriteString("\n")
	}

	for _, internalFunc := range internalFunctions {
		builder.WriteString(internalFunc + "\n\n")
	}

	for _, fn := range gg.generator.Functions {
		builder.WriteString(fmt.Sprintf("//export %s\n%s\n", fn.Name, fn.GoFunction))
	}

	// Generate struct declarations for classes
	for _, class := range gg.generator.Classes {
		builder.WriteString(fmt.Sprintf("type %s struct {\n", class.GoStruct))
		for _, prop := range class.Properties {
			builder.WriteString(fmt.Sprintf("	%s %s\n", prop.Name, prop.GoType))
		}
		builder.WriteString("}\n\n")
	}

	// Generate object registry if we have classes
	if len(gg.generator.Classes) > 0 {
		builder.WriteString(`
var objectRegistry = make(map[uint64]interface{})
var nextObjectID uint64 = 1
var registryMutex sync.RWMutex

//export registerGoObject
func registerGoObject(obj unsafe.Pointer) uint64 {
	registryMutex.Lock()
	defer registryMutex.Unlock()
	
	id := nextObjectID
	nextObjectID++
	objectRegistry[id] = obj
	return id
}

//export getGoObject
func getGoObject(id uint64) unsafe.Pointer {
	registryMutex.RLock()
	defer registryMutex.RUnlock()
	
	if obj, exists := objectRegistry[id]; exists {
		return obj.(unsafe.Pointer)
	}
	return nil
}

//export removeGoObject
func removeGoObject(id uint64) {
	registryMutex.Lock()
	defer registryMutex.Unlock()
	
	delete(objectRegistry, id)
}

`)
	}

	for _, class := range gg.generator.Classes {
		builder.WriteString(fmt.Sprintf(`//export create_%s_object
func create_%s_object() uint64 {
	obj := &%s{}
	return registerGoObject(unsafe.Pointer(obj))
}

`, class.GoStruct, class.GoStruct, class.GoStruct))

		// Generate property getters and setters
		for _, prop := range class.Properties {
			// Getter
			builder.WriteString(fmt.Sprintf("//export get_%s_%s_property\n", class.Name, prop.Name))
			if prop.GoType == "string" {
				builder.WriteString(fmt.Sprintf(`func get_%s_%s_property(objectID uint64) string {
	objPtr := getGoObject(objectID)
	obj := (*%s)(objPtr)
	return obj.%s
}

`, class.Name, prop.Name, class.GoStruct, prop.Name))
			} else {
				goReturnType := gg.phpTypeToGoType(prop.Type)
				builder.WriteString(fmt.Sprintf(`func get_%s_%s_property(objectID uint64) %s {
	objPtr := getGoObject(objectID)
	obj := (*%s)(objPtr)
	return obj.%s
}

`, class.Name, prop.Name, goReturnType, goReturnType, class.GoStruct, prop.Name))
			}

			// Setter
			builder.WriteString(fmt.Sprintf("//export set_%s_%s_property\n", class.Name, prop.Name))
			goType := gg.phpTypeToGoType(prop.Type)
			builder.WriteString(fmt.Sprintf(`func set_%s_%s_property(objectID uint64, value %s) {
	objPtr := getGoObject(objectID)
	obj := (*%s)(objPtr)
	obj.%s = value
}

`, class.Name, prop.Name, goType, class.GoStruct, prop.Name))
		}

		// Generate original methods from the source
		for _, method := range class.Methods {
			if method.GoFunction != "" {
				builder.WriteString(method.GoFunction)
				builder.WriteString("\n\n")
			}
		}

		// Generate method wrappers
		for _, method := range class.Methods {
			builder.WriteString(fmt.Sprintf("//export %s_wrapper\n", method.Name))
			builder.WriteString(gg.generateMethodWrapper(method, class))
			builder.WriteString("\n")
		}
	}

	return builder.String(), nil
}

func (gg *GoFileGenerator) generateMethodWrapper(method ClassMethod, class PHPClass) string {
	var builder strings.Builder

	builder.WriteString(fmt.Sprintf("func %s_wrapper(objectID uint64", method.Name))

	// Use C types for parameters to match what the C template passes
	for _, param := range method.Params {
		if param.Type == "string" {
			builder.WriteString(fmt.Sprintf(", %s *C.zend_string", param.Name))
		} else {
			goType := gg.phpTypeToGoType(param.Type)
			builder.WriteString(fmt.Sprintf(", %s %s", param.Name, goType))
		}
	}

	if method.ReturnType != "void" {
		goReturnType := gg.phpTypeToGoType(method.ReturnType)
		builder.WriteString(fmt.Sprintf(") %s {\n", goReturnType))
	} else {
		builder.WriteString(") {\n")
	}

	builder.WriteString("	objPtr := getGoObject(objectID)\n")
	builder.WriteString(fmt.Sprintf("	obj := (*%s)(objPtr)\n", class.GoStruct))

	builder.WriteString("	")
	if method.ReturnType != "void" {
		builder.WriteString("return ")
	}

	builder.WriteString(fmt.Sprintf("obj.%s(", gg.goMethodName(method.Name)))

	for i, param := range method.Params {
		if i > 0 {
			builder.WriteString(", ")
		}
		if param.Type == "string" {
			builder.WriteString(fmt.Sprintf("C.GoStringN(C.ZSTR_VAL(%s), C.int(C.ZSTR_LEN(%s)))", param.Name, param.Name))
		} else {
			builder.WriteString(param.Name)
		}
	}

	builder.WriteString(")\n")
	builder.WriteString("}")

	return builder.String()
}

type GoMethodSignature struct {
	MethodName string
	Params     []GoParameter
	ReturnType string
}

type GoParameter struct {
	Name string
	Type string
}

func (gg *GoFileGenerator) parseGoMethodSignature(goFunction string) (*GoMethodSignature, error) {
	// Simple parsing of Go function signature
	// Example: "func (mc *MyClass) SetVersion(version string) {"
	lines := strings.Split(goFunction, "\n")
	if len(lines) == 0 {
		return nil, fmt.Errorf("empty function")
	}

	funcLine := strings.TrimSpace(lines[0])

	// Extract method name and parameters
	if !strings.HasPrefix(funcLine, "func ") {
		return nil, fmt.Errorf("not a function")
	}

	// Find the method name
	parts := strings.Split(funcLine, ")")
	if len(parts) < 2 {
		return nil, fmt.Errorf("invalid function signature")
	}

	// Get the part after the receiver
	methodPart := strings.TrimSpace(parts[1])

	// Extract method name
	spaceIndex := strings.Index(methodPart, "(")
	if spaceIndex == -1 {
		return nil, fmt.Errorf("no parameters found")
	}

	methodName := strings.TrimSpace(methodPart[:spaceIndex])

	// Extract parameters
	paramStart := strings.Index(methodPart, "(")
	paramEnd := strings.LastIndex(methodPart, ")")
	if paramStart == -1 || paramEnd == -1 || paramStart >= paramEnd {
		return nil, fmt.Errorf("invalid parameter section")
	}

	paramSection := methodPart[paramStart+1 : paramEnd]
	var params []GoParameter

	if strings.TrimSpace(paramSection) != "" {
		paramParts := strings.Split(paramSection, ",")
		for _, paramPart := range paramParts {
			paramPart = strings.TrimSpace(paramPart)
			if paramPart == "" {
				continue
			}

			// Parse "name type" format
			parts := strings.Fields(paramPart)
			if len(parts) >= 2 {
				params = append(params, GoParameter{
					Name: parts[0],
					Type: strings.Join(parts[1:], " "),
				})
			}
		}
	}

	// Extract return type
	returnType := ""
	if strings.Contains(methodPart, ") ") && !strings.HasSuffix(methodPart, ") {") {
		afterParen := strings.Split(methodPart, ") ")
		if len(afterParen) > 1 {
			returnPart := strings.TrimSpace(afterParen[1])
			if strings.HasSuffix(returnPart, " {") {
				returnType = strings.TrimSpace(returnPart[:len(returnPart)-2])
			}
		}
	}

	return &GoMethodSignature{
		MethodName: methodName,
		Params:     params,
		ReturnType: returnType,
	}, nil
}

func (gg *GoFileGenerator) generateMethodWrapperFallback(method ClassMethod, class PHPClass) string {
	var builder strings.Builder

	builder.WriteString(fmt.Sprintf("func %s_wrapper(objectID uint64", method.Name))

	for _, param := range method.Params {
		goType := gg.phpTypeToGoType(param.Type)
		builder.WriteString(fmt.Sprintf(", %s %s", param.Name, goType))
	}

	if method.ReturnType != "void" {
		goReturnType := gg.phpTypeToGoType(method.ReturnType)
		builder.WriteString(fmt.Sprintf(") %s {\n", goReturnType))
	} else {
		builder.WriteString(") {\n")
	}

	builder.WriteString("	objPtr := getGoObject(objectID)\n")
	builder.WriteString(fmt.Sprintf("	obj := (*%s)(objPtr)\n", class.GoStruct))

	builder.WriteString("	")
	if method.ReturnType != "void" {
		builder.WriteString("return ")
	}

	builder.WriteString(fmt.Sprintf("obj.%s(", gg.goMethodName(method.Name)))

	for i, param := range method.Params {
		if i > 0 {
			builder.WriteString(", ")
		}
		builder.WriteString(param.Name)
	}

	builder.WriteString(")\n")
	builder.WriteString("}")

	return builder.String()
}

func (gg *GoFileGenerator) phpTypeToGoType(phpType string) string {
	typeMap := map[string]string{
		"string": "string",
		"int":    "int",
		"float":  "float64",
		"bool":   "bool",
		"array":  "[]interface{}",
		"mixed":  "interface{}",
		"void":   "",
	}

	if goType, exists := typeMap[phpType]; exists {
		return goType
	}

	return "interface{}"
}

func (gg *GoFileGenerator) goMethodName(phpMethodName string) string {
	if len(phpMethodName) == 0 {
		return phpMethodName
	}

	return strings.ToUpper(phpMethodName[:1]) + phpMethodName[1:]
}
