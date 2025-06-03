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
	filename := filepath.Join(gg.generator.BuildDir, gg.generator.BaseName+"_ext.go")
	content, err := gg.buildContent()
	if err != nil {
		return fmt.Errorf("building Go file content: %w", err)
	}
	return WriteFile(filename, content)
}

func (gg *GoFileGenerator) buildContent() (string, error) {
	sourceAnalyzer := SourceAnalyzer{}
	packageName, imports, internalFunctions, err := sourceAnalyzer.Analyze(gg.generator.SourceFile)
	if err != nil {
		return "", fmt.Errorf("analyzing source file: %w", err)
	}

	var builder strings.Builder

	cleanPackageName := SanitizePackageName(gg.generator.BaseName)
	builder.WriteString(fmt.Sprintf("package %s\n\n", cleanPackageName))
	builder.WriteString(fmt.Sprintf("/*\n#include <stdlib.h>\n#include \"%s.h\"\n*/\nimport \"C\"\n", gg.generator.BaseName))

	for _, imp := range imports {
		if !strings.Contains(imp, `"C"`) && !strings.Contains(imp, "github.com/dunglas/frankenphp/internal/extensions/types") {
			if strings.Contains(imp, packageName) {
				imp = strings.ReplaceAll(imp, packageName, "types")
			}
			builder.WriteString(fmt.Sprintf("import %s\n", imp))
		}
	}

	builder.WriteString("\nfunc init() {\n\tC.register_extension()\n}\n\n")

	for _, internalFunc := range internalFunctions {
		builder.WriteString(internalFunc + "\n\n")
	}

	for _, fn := range gg.generator.Functions {
		builder.WriteString(fmt.Sprintf("//export %s\n%s\n", fn.Name, fn.GoFunction))
	}

	return builder.String(), nil
}
