package extgen

import (
	"strconv"
	"strings"
)

type PHPFunction struct {
	Name             string
	Signature        string
	GoFunction       string
	Params           []Parameter
	ReturnType       string
	IsReturnNullable bool
	LineNumber       int
}

type Parameter struct {
	Name         string
	Type         string
	IsNullable   bool
	DefaultValue string
	HasDefault   bool
}

type PHPClass struct {
	Name       string
	GoStruct   string
	Properties []ClassProperty
	Methods    []ClassMethod
}

type ClassMethod struct {
	Name             string
	PHPName          string
	Signature        string
	GoFunction       string
	Params           []Parameter
	ReturnType       string
	IsReturnNullable bool
	LineNumber       int
	ClassName        string // used by the "//export_php:method" directive
}

type ClassProperty struct {
	Name       string
	Type       string
	GoType     string
	IsNullable bool
}

type PHPConstant struct {
	Name       string
	Value      string
	Type       string // "int", "string", "bool", "float"
	IsIota     bool
	LineNumber int
	ClassName  string // empty for global constants, set for class constants
}

// CValue returns the constant value in C-compatible format
func (c PHPConstant) CValue() string {
	if c.Type != "int" {
		return c.Value
	}

	if strings.HasPrefix(c.Value, "0o") {
		if val, err := strconv.ParseInt(c.Value, 0, 64); err == nil {
			return strconv.FormatInt(val, 10)
		}
	}

	return c.Value
}

// CReturnType returns the C type for method return type
func (m ClassMethod) CReturnType() string {
	return phpTypeToCType(m.ReturnType)
}

// CGOReturnType returns the CGO type for method return type
func (m ClassMethod) CGOReturnType() string {
	return phpTypeToCGOType(m.ReturnType)
}

// CType returns the C type for parameter
func (p Parameter) CType() string {
	baseType := phpTypeToCType(p.Type)
	// string is provided by a zend_string*, nullable by nature
	if p.IsNullable && p.Type != "string" {
		return baseType + "*"
	}
	return baseType
}

// CGOType returns the CGO type for parameter
func (p Parameter) CGOType() string {
	baseType := phpTypeToCGOType(p.Type)
	if p.IsNullable && p.Type != "string" {
		return "*" + baseType
	}
	return baseType
}

// phpTypeToCType converts PHP types to C types for headers
func phpTypeToCType(phpType string) string {
	typeMap := map[string]string{
		"void":   "void",
		"string": "void*",
		"int":    "int64_t",
		"float":  "double",
		"bool":   "int",
		"array":  "void*",
		"mixed":  "void*",
	}

	if cType, exists := typeMap[phpType]; exists {
		return cType
	}

	return "void*"
}

// phpTypeToCGOType converts PHP types to CGO types for headers
func phpTypeToCGOType(phpType string) string {
	typeMap := map[string]string{
		"void":   "void",
		"string": "zend_string*",
		"int":    "int64",
		"float":  "float64",
		"bool":   "bool",
		"array":  "GoSlice",
		"mixed":  "GoInterface",
	}

	if cgoType, exists := typeMap[phpType]; exists {
		return cgoType
	}

	return "GoInterface"
}
