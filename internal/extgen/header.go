package extgen

import (
	_ "embed"
	"fmt"
	"path/filepath"
	"strings"
)

//go:embed templates/extension.h.tmpl
var hFileContent string

type HeaderGenerator struct {
	generator *Generator
}

func (hg *HeaderGenerator) generate() error {
	filename := filepath.Join(hg.generator.BuildDir, hg.generator.BaseName+".h")
	content := hg.buildContent()
	return WriteFile(filename, content)
}

func (hg *HeaderGenerator) buildContent() string {
	headerGuard := strings.Map(func(r rune) rune {
		if r >= 'A' && r <= 'Z' || r >= 'a' && r <= 'z' || r >= '0' && r <= '9' {
			return r
		}

		return '_'
	}, hg.generator.BaseName)

	headerGuard = strings.ToUpper(headerGuard) + "_H"

	return fmt.Sprintf(hFileContent, headerGuard, headerGuard)
}
