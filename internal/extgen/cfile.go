package extgen

import (
	"bytes"
	_ "embed"
	"github.com/iancoleman/strcase"
	"path/filepath"
	"strings"
	"text/template"
)

//go:embed templates/extension.c.tpl
var cFileContent string

type CFileGenerator struct {
	generator *Generator
}

type CTemplateData struct {
	BaseName  string
	Functions []PHPFunction
	Classes   []PHPClass
	Constants []PHPConstant
	Version   string
}

func (cg *CFileGenerator) generate() error {
	filename := filepath.Join(cg.generator.BuildDir, cg.generator.BaseName+".c")
	content, err := cg.buildContent()
	if err != nil {
		return err
	}
	return WriteFile(filename, content)
}

func (cg *CFileGenerator) buildContent() (string, error) {
	var builder strings.Builder

	templateContent, err := cg.getTemplateContent()
	if err != nil {
		return "", err
	}
	builder.WriteString(templateContent)

	for _, fn := range cg.generator.Functions {
		fnGen := PHPFuncGenerator{}
		builder.WriteString(fnGen.generate(fn))
	}

	return builder.String(), nil
}

func (cg *CFileGenerator) getTemplateContent() (string, error) {
	funcMap := template.FuncMap{
		"ToLower": strcase.ToLowerCamel,
	}
	tmpl, err := template.New("cfile").Funcs(funcMap).Parse(cFileContent)
	if err != nil {
		return "", err
	}

	data := CTemplateData{
		BaseName:  cg.generator.BaseName,
		Functions: cg.generator.Functions,
		Classes:   cg.generator.Classes,
		Constants: cg.generator.Constants,
		Version:   "1.0.0",
	}

	var buf bytes.Buffer
	err = tmpl.Execute(&buf, data)
	if err != nil {
		return "", err
	}

	return buf.String(), nil
}
