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
`, cleanPackageName, gg.generator.BaseName))

	for _, imp := range imports {
		if imp == `"C"` {
			continue
		}

		builder.WriteString(fmt.Sprintf("import %s\n", imp))
	}

	builder.WriteString("\nfunc init() {\n\tC.register_extension()\n}\n\n") // TODO update with the new frankenphp func!

	for _, internalFunc := range internalFunctions {
		builder.WriteString(internalFunc + "\n\n")
	}

	for _, fn := range gg.generator.Functions {
		builder.WriteString(fmt.Sprintf("//export %s\n%s\n", fn.Name, fn.GoFunction))
	}

	return builder.String(), nil
}
