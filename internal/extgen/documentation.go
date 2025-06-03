package extgen

import (
	"fmt"
	"path/filepath"
	"strings"
)

type DocumentationGenerator struct {
	generator *Generator
}

func (dg *DocumentationGenerator) generate() error {
	filename := filepath.Join(dg.generator.BuildDir, "README.md")
	content := dg.generateMarkdown()
	return WriteFile(filename, content)
}

func (dg *DocumentationGenerator) generateMarkdown() string {
	var builder strings.Builder

	builder.WriteString(fmt.Sprintf("# %s Extension\n\n", dg.generator.BaseName))
	builder.WriteString("Auto-generated PHP extension from Go code.\n\n")

	if len(dg.generator.Functions) > 0 {
		builder.WriteString("## Functions\n\n")
		for _, fn := range dg.generator.Functions {
			builder.WriteString(fmt.Sprintf("### %s\n\n", fn.Name))
			builder.WriteString(fmt.Sprintf("```php\n%s\n```\n\n", fn.Signature))

			if len(fn.Params) > 0 {
				builder.WriteString("**Parameters:**\n\n")
				for _, param := range fn.Params {
					nullable := ""
					if param.IsNullable {
						nullable = " (nullable)"
					}
					defaultVal := ""
					if param.HasDefault {
						defaultVal = fmt.Sprintf(" (default: %s)", param.DefaultValue)
					}
					builder.WriteString(fmt.Sprintf("- `%s` (%s)%s%s\n", param.Name, param.Type, nullable, defaultVal))
				}
				builder.WriteString("\n")
			}

			nullable := ""
			if fn.IsReturnNullable {
				nullable = " (nullable)"
			}
			builder.WriteString(fmt.Sprintf("**Returns:** %s%s\n\n", fn.ReturnType, nullable))
		}
	}

	if len(dg.generator.Classes) > 0 {
		builder.WriteString("## Classes\n\n")
		for _, class := range dg.generator.Classes {
			builder.WriteString(fmt.Sprintf("### %s\n\n", class.Name))

			if len(class.Properties) > 0 {
				builder.WriteString("**Properties:**\n\n")
				for _, prop := range class.Properties {
					nullable := ""
					if prop.IsNullable {
						nullable = " (nullable)"
					}
					builder.WriteString(fmt.Sprintf("- `%s`: %s%s\n", prop.Name, prop.Type, nullable))
				}
				builder.WriteString("\n")
			}
		}
	}

	return builder.String()
}
