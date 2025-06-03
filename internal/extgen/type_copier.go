package extgen

import (
	_ "embed"
	"fmt"
	"path/filepath"
)

//go:embed "c_types.h"
var typesHSrc string

type TypeCopier struct {
	generator *Generator
}

func newTypeCopier(g *Generator) *TypeCopier {
	return &TypeCopier{generator: g}
}

func (tc *TypeCopier) copy() error {
	typesHDest := filepath.Join(tc.generator.BuildDir, "types.h")

	if err := WriteFile(typesHDest, typesHSrc); err != nil {
		return fmt.Errorf("copying types.h: %w", err)
	}

	return nil
}
