package extgen

import (
	"bytes"
	_ "embed"
	"github.com/iancoleman/strcase"
	"path/filepath"
	"regexp"
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

	return cg.cleanupWhitespace(buf.String()), nil
}

// cleanupWhitespace removes excessive blank lines and trailing whitespace
func (cg *CFileGenerator) cleanupWhitespace(content string) string {
	lines := strings.Split(content, "\n")
	for i, line := range lines {
		lines[i] = strings.TrimRight(line, " \t")
	}

	content = strings.Join(lines, "\n")

	blankBeforeBrace := regexp.MustCompile(`\n\n+}`)
	content = blankBeforeBrace.ReplaceAllString(content, "\n}")

	multipleBlankLines := regexp.MustCompile(`\n\n\n+`)
	content = multipleBlankLines.ReplaceAllString(content, "\n\n")

	content = strings.TrimLeft(content, "\n")
	content = strings.TrimRight(content, "\n") + "\n"

	return content
}
