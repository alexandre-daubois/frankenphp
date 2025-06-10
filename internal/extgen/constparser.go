package extgen

import (
	"bufio"
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"
)

var constRegex = regexp.MustCompile(`//\s*export_php:const$`)
var constDeclRegex = regexp.MustCompile(`const\s+(\w+)\s*=\s*(.+)`)

type ConstantParser struct {
	constRegex     *regexp.Regexp
	constDeclRegex *regexp.Regexp
}

func NewConstantParserWithDefRegex() *ConstantParser {
	return &ConstantParser{
		constRegex:     constRegex,
		constDeclRegex: constDeclRegex,
	}
}

func (cp *ConstantParser) parse(filename string) ([]PHPConstant, error) {
	file, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var constants []PHPConstant
	scanner := bufio.NewScanner(file)

	lineNumber := 0
	expectConstDecl := false
	currentConstantValue := 0

	for scanner.Scan() {
		lineNumber++
		line := strings.TrimSpace(scanner.Text())

		if cp.constRegex.MatchString(line) {
			expectConstDecl = true
			continue
		}

		if expectConstDecl && strings.HasPrefix(line, "const ") {
			matches := cp.constDeclRegex.FindStringSubmatch(line)
			if len(matches) == 3 {
				name := matches[1]
				value := strings.TrimSpace(matches[2])

				constant := PHPConstant{
					Name:       name,
					Value:      value,
					IsIota:     value == "iota",
					LineNumber: lineNumber,
				}

				constant.Type = determineConstantType(value)

				if constant.IsIota {
					// affect a default value because user didn't give one
					constant.Value = fmt.Sprintf("%d", currentConstantValue)
					constant.Type = "int"
					currentConstantValue++
				}

				constants = append(constants, constant)
			} else {
				return nil, fmt.Errorf("invalid constant declaration at line %d: %s", lineNumber, line)
			}
			expectConstDecl = false
		} else if expectConstDecl && !strings.HasPrefix(line, "//") && line != "" {
			// we expected a const declaration but found something else, reset
			expectConstDecl = false
		}
	}

	return constants, scanner.Err()
}

// determineConstantType analyzes the value and determines its type
func determineConstantType(value string) string {
	value = strings.TrimSpace(value)

	if (strings.HasPrefix(value, "\"") && strings.HasSuffix(value, "\"")) ||
		(strings.HasPrefix(value, "`") && strings.HasSuffix(value, "`")) {
		return "string"
	}

	if value == "true" || value == "false" {
		return "bool"
	}

	// check for integer literals, including hex, octal, binary
	if _, err := strconv.ParseInt(value, 0, 64); err == nil {
		return "int"
	}

	if _, err := strconv.ParseFloat(value, 64); err == nil {
		return "float"
	}

	return "int"
}
