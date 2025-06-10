package extgen

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/iancoleman/strcase"
)

type StubGenerator struct {
	generator *Generator
}

func (sg *StubGenerator) generate() error {
	filename := filepath.Join(sg.generator.BuildDir, sg.generator.BaseName+".stub.php")
	content := sg.buildContent()
	return WriteFile(filename, content)
}

func (sg *StubGenerator) buildContent() string {
	var builder strings.Builder

	builder.WriteString("<?php\n\n/** @generate-class-entries */\n\n")

	for _, constant := range sg.generator.Constants {
		if constant.IsIota {
			// For iota constants, use @cvalue annotation to let PHP generate the value
			builder.WriteString(fmt.Sprintf(`/**
 * @var int
 * @cvalue %s
 */
const %s = UNKNOWN;

`, constant.Name, constant.Name))
		} else {
			phpType := getPhpTypeAnnotation(constant.Type)
			builder.WriteString(fmt.Sprintf(`/**
 * @var %s
 */
const %s = %s;

`, phpType, constant.Name, constant.Value))
		}
	}

	for _, fn := range sg.generator.Functions {
		builder.WriteString(fmt.Sprintf("function %s {}\n\n", fn.Signature))
	}

	for _, class := range sg.generator.Classes {
		builder.WriteString(fmt.Sprintf("class %s {\n", class.Name))

		for _, prop := range class.Properties {
			nullable := ""
			if prop.IsNullable {
				nullable = "?"
			}
			builder.WriteString(fmt.Sprintf("    public %s%s $%s;\n",
				nullable, prop.Type, strcase.ToLowerCamel(prop.Name)))
		}

		builder.WriteString("\n    public function __construct() {}\n")
		builder.WriteString("}\n\n")
	}

	return builder.String()
}

// getPhpTypeAnnotation converts Go constant type to PHP type annotation
func getPhpTypeAnnotation(goType string) string {
	switch goType {
	case "string":
		return "string"
	case "bool":
		return "bool"
	case "float":
		return "float"
	case "int":
		return "int"
	default:
		return "int" // fallback
	}
}
