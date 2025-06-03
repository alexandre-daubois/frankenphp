package extgen

import (
	_ "embed"
	"fmt"
	"path/filepath"
	"strings"
)

//go:embed templates/extension.c.tmpl
var cFileContent string

type CFileGenerator struct {
	generator *Generator
}

func (cg *CFileGenerator) generate() error {
	filename := filepath.Join(cg.generator.BuildDir, cg.generator.BaseName+".c")
	content := cg.buildContent()
	return WriteFile(filename, content)
}

func (cg *CFileGenerator) buildContent() string {
	var builder strings.Builder

	builder.WriteString(cg.getTemplateContent())

	for _, fn := range cg.generator.Functions {
		fnGen := PHPFuncGenerator{}
		builder.WriteString(fnGen.generate(fn))
	}

	if len(cg.generator.Classes) > 0 {
		classGen := PHPClassGenerator{}
		builder.WriteString(classGen.generate(cg.generator.Classes))
	}

	return builder.String()
}

func (cg *CFileGenerator) getTemplateContent() string {
	return fmt.Sprintf(
		cFileContent,
		cg.generator.BaseName,
		cg.generator.BaseName,
		cg.generator.BaseName,
		cg.generator.BaseName,
		cg.generator.BaseName,
		cg.generator.BaseName,
		cg.generator.BaseName,
	)
}
