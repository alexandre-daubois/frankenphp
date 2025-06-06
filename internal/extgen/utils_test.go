package extgen

import (
	"os"
	"path/filepath"
	"testing"
)

func TestWriteFile(t *testing.T) {
	tests := []struct {
		name        string
		filename    string
		content     string
		expectError bool
	}{
		{
			name:        "write simple file",
			filename:    "test.txt",
			content:     "hello world",
			expectError: false,
		},
		{
			name:        "write empty file",
			filename:    "empty.txt",
			content:     "",
			expectError: false,
		},
		{
			name:        "write file with special characters",
			filename:    "special.txt",
			content:     "hello\nworld\t!@#$%^&*()",
			expectError: false,
		},
		{
			name:        "write to invalid directory",
			filename:    "/nonexistent/directory/file.txt",
			content:     "test",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var filename string
			if !tt.expectError {
				tempDir := t.TempDir()
				filename = filepath.Join(tempDir, tt.filename)
			} else {
				filename = tt.filename
			}

			err := WriteFile(filename, tt.content)

			if tt.expectError {
				if err == nil {
					t.Errorf("WriteFile() expected error but got none")
				}
				return
			}

			if err != nil {
				t.Errorf("WriteFile() unexpected error: %v", err)
				return
			}

			content, err := os.ReadFile(filename)
			if err != nil {
				t.Errorf("Failed to read written file: %v", err)
				return
			}

			if string(content) != tt.content {
				t.Errorf("WriteFile() content mismatch. Expected: %q, got: %q", tt.content, string(content))
			}

			info, err := os.Stat(filename)
			if err != nil {
				t.Errorf("Failed to stat file: %v", err)
				return
			}

			expectedMode := os.FileMode(0644)
			if info.Mode().Perm() != expectedMode {
				t.Errorf("WriteFile() wrong permissions. Expected: %v, got: %v", expectedMode, info.Mode().Perm())
			}
		})
	}
}

func TestReadFile(t *testing.T) {
	tests := []struct {
		name        string
		content     string
		expectError bool
	}{
		{
			name:        "read simple file",
			content:     "hello world",
			expectError: false,
		},
		{
			name:        "read empty file",
			content:     "",
			expectError: false,
		},
		{
			name:        "read file with special characters",
			content:     "hello\nworld\t!@#$%^&*()",
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tempDir := t.TempDir()
			filename := filepath.Join(tempDir, "test.txt")

			err := os.WriteFile(filename, []byte(tt.content), 0644)
			if err != nil {
				t.Fatalf("Failed to create test file: %v", err)
			}

			content, err := ReadFile(filename)

			if tt.expectError {
				if err == nil {
					t.Errorf("ReadFile() expected error but got none")
				}
				return
			}

			if err != nil {
				t.Errorf("ReadFile() unexpected error: %v", err)
				return
			}

			if content != tt.content {
				t.Errorf("ReadFile() content mismatch. Expected: %q, got: %q", tt.content, content)
			}
		})
	}

	t.Run("read nonexistent file", func(t *testing.T) {
		_, err := ReadFile("/nonexistent/file.txt")
		if err == nil {
			t.Errorf("ReadFile() expected error for nonexistent file but got none")
		}
	})
}

func TestSanitizePackageName(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "simple valid name",
			input:    "mypackage",
			expected: "mypackage",
		},
		{
			name:     "name with hyphens",
			input:    "my-package",
			expected: "my_package",
		},
		{
			name:     "name with dots",
			input:    "my.package",
			expected: "my_package",
		},
		{
			name:     "name with both hyphens and dots",
			input:    "my-package.name",
			expected: "my_package_name",
		},
		{
			name:     "name starting with number",
			input:    "123package",
			expected: "_123package",
		},
		{
			name:     "name starting with underscore",
			input:    "_package",
			expected: "_package",
		},
		{
			name:     "name starting with letter",
			input:    "Package",
			expected: "Package",
		},
		{
			name:     "name starting with special character",
			input:    "@package",
			expected: "_@package",
		},
		{
			name:     "complex name",
			input:    "123my-complex.package@name",
			expected: "_123my_complex_package@name",
		},
		{
			name:     "empty string",
			input:    "",
			expected: "",
		},
		{
			name:     "single character letter",
			input:    "a",
			expected: "a",
		},
		{
			name:     "single character number",
			input:    "1",
			expected: "_1",
		},
		{
			name:     "single character underscore",
			input:    "_",
			expected: "_",
		},
		{
			name:     "single character special",
			input:    "@",
			expected: "_@",
		},
		{
			name:     "multiple consecutive hyphens",
			input:    "my--package",
			expected: "my__package",
		},
		{
			name:     "multiple consecutive dots",
			input:    "my..package",
			expected: "my__package",
		},
		{
			name:     "mixed case with special chars",
			input:    "MyPackage-Name.Version",
			expected: "MyPackage_Name_Version",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := SanitizePackageName(tt.input)
			if result != tt.expected {
				t.Errorf("SanitizePackageName(%q) = %q, expected %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestIsLetter(t *testing.T) {
	tests := []struct {
		name     string
		input    rune
		expected bool
	}{
		{
			name:     "lowercase letter",
			input:    'a',
			expected: true,
		},
		{
			name:     "uppercase letter",
			input:    'A',
			expected: true,
		},
		{
			name:     "lowercase z",
			input:    'z',
			expected: true,
		},
		{
			name:     "uppercase Z",
			input:    'Z',
			expected: true,
		},
		{
			name:     "digit",
			input:    '1',
			expected: false,
		},
		{
			name:     "underscore",
			input:    '_',
			expected: false,
		},
		{
			name:     "hyphen",
			input:    '-',
			expected: false,
		},
		{
			name:     "space",
			input:    ' ',
			expected: false,
		},
		{
			name:     "special character",
			input:    '@',
			expected: false,
		},
		{
			name:     "unicode letter",
			input:    'ñ',
			expected: false,
		},
		{
			name:     "tab",
			input:    '\t',
			expected: false,
		},
		{
			name:     "newline",
			input:    '\n',
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isLetter(tt.input)
			if result != tt.expected {
				t.Errorf("isLetter(%q) = %v, expected %v", tt.input, result, tt.expected)
			}
		})
	}
}

func BenchmarkSanitizePackageName(b *testing.B) {
	testCases := []string{
		"simple",
		"my-package",
		"my.package.name",
		"123complex-package.name@version",
		"very-long-package-name-with-many-special-characters.and.dots",
	}

	for _, tc := range testCases {
		b.Run(tc, func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				SanitizePackageName(tc)
			}
		})
	}
}

func BenchmarkIsLetter(b *testing.B) {
	testRunes := []rune{'a', 'Z', '1', '_', '@', 'ñ'}

	for _, r := range testRunes {
		b.Run(string(r), func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				isLetter(r)
			}
		})
	}
}
