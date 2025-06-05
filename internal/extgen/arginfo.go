package extgen

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

type ArginfoGenerator struct {
	generator *Generator
}

func (ag *ArginfoGenerator) Generate() error {
	genStubPath := os.Getenv("GEN_STUB_SCRIPT")
	if genStubPath == "" {
		return fmt.Errorf("GEN_STUB_SCRIPT environment variable is not set")
	}

	stubFile := ag.generator.BaseName + ".stub.php"
	cmd := exec.Command("php", genStubPath, filepath.Join(ag.generator.BuildDir, stubFile))

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("running gen_stub script: %w", err)
	}

	return ag.fixArginfoFile(stubFile)
}

func (ag *ArginfoGenerator) fixArginfoFile(stubFile string) error {
	arginfoFile := strings.TrimSuffix(stubFile, ".stub.php") + "_arginfo.h"
	arginfoPath := filepath.Join(ag.generator.BuildDir, arginfoFile)

	content, err := ReadFile(arginfoPath)
	if err != nil {
		return fmt.Errorf("reading arginfo file: %w", err)
	}

	// TODO: Fix the zend_register_internal_class_with_flags issue
	fixedContent := strings.ReplaceAll(content,
		"zend_register_internal_class_with_flags(&ce, NULL, 0)",
		"zend_register_internal_class(&ce)")

	return WriteFile(arginfoPath, fixedContent)
}
