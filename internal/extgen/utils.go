package extgen

import (
	"os"
	"strings"
)

func WriteFile(filename, content string) error {
	return os.WriteFile(filename, []byte(content), 0644)
}

func ReadFile(filename string) (string, error) {
	content, err := os.ReadFile(filename)
	if err != nil {
		return "", err
	}
	return string(content), nil
}

// EXPERIMENTAL
func SanitizePackageName(name string) string {
	sanitized := strings.ReplaceAll(name, "-", "_")
	sanitized = strings.ReplaceAll(sanitized, ".", "_")

	if len(sanitized) > 0 && !isLetter(rune(sanitized[0])) && sanitized[0] != '_' {
		sanitized = "_" + sanitized
	}

	return sanitized
}

func isLetter(r rune) bool {
	return (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z')
}
