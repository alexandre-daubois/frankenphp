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
