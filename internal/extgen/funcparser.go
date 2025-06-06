package extgen

import (
	"bufio"
	"fmt"
	"os"
	"regexp"
	"strings"
)

var phpFuncRegex = regexp.MustCompile(`//\s*export_php:?\s*(?:function\s+)?([^{}\n]+)(?:\s*\{\s*\})?`)
var signatureRegex = regexp.MustCompile(`(\w+)\s*\(([^)]*)\)\s*:\s*(\??[\w|]+)`)
var typeNameRegex = regexp.MustCompile(`(\??[\w|]+)\s+\$?(\w+)`)

type FuncParser struct {
	phpFuncRegex *regexp.Regexp
}

func NewFuncParserDefRegex() *FuncParser {
	return &FuncParser{
		phpFuncRegex: phpFuncRegex,
	}
}

func (fp *FuncParser) parse(filename string) ([]PHPFunction, error) {
	file, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var functions []PHPFunction
	scanner := bufio.NewScanner(file)
	var currentPHPFunc *PHPFunction
	validator := NewValidator()

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		if matches := fp.phpFuncRegex.FindStringSubmatch(line); matches != nil {
			signature := strings.TrimSpace(matches[1])
			phpFunc, err := fp.parseSignature(signature)
			if err != nil {
				fmt.Printf("Warning: Error parsing signature '%s': %v\n", signature, err)
				continue
			}

			if err := validator.ValidateFunction(*phpFunc); err != nil {
				fmt.Printf("Warning: Invalid function '%s': %v\n", phpFunc.Name, err)
				continue
			}

			currentPHPFunc = phpFunc
		}

		if currentPHPFunc != nil && strings.HasPrefix(line, "func ") {
			goFunc, err := fp.extractGoFunction(scanner, line)
			if err != nil {
				return nil, fmt.Errorf("extracting Go function: %w", err)
			}
			currentPHPFunc.GoFunction = goFunc
			functions = append(functions, *currentPHPFunc)
			currentPHPFunc = nil
		}
	}

	return functions, scanner.Err()
}

func (fp *FuncParser) extractGoFunction(scanner *bufio.Scanner, firstLine string) (string, error) {
	goFunc := firstLine + "\n"
	braceCount := 1

	for scanner.Scan() {
		line := scanner.Text()
		goFunc += line + "\n"

		for _, char := range line {
			switch char {
			case '{':
				braceCount++
			case '}':
				braceCount--
			}
		}

		if braceCount == 0 {
			break
		}
	}

	return goFunc, nil
}

func (fp *FuncParser) parseSignature(signature string) (*PHPFunction, error) {
	matches := signatureRegex.FindStringSubmatch(signature)

	if len(matches) != 4 {
		return nil, fmt.Errorf("invalid signature format")
	}

	name := matches[1]
	paramsStr := strings.TrimSpace(matches[2])
	returnTypeStr := strings.TrimSpace(matches[3])

	isReturnNullable := strings.HasPrefix(returnTypeStr, "?")
	returnType := strings.TrimPrefix(returnTypeStr, "?")

	var params []Parameter
	if paramsStr != "" {
		paramParts := strings.Split(paramsStr, ",")
		for _, part := range paramParts {
			param, err := fp.parseParameter(strings.TrimSpace(part))
			if err != nil {
				return nil, fmt.Errorf("parsing parameter '%s': %w", part, err)
			}
			params = append(params, param)
		}
	}

	return &PHPFunction{
		Name:             name,
		Signature:        signature,
		Params:           params,
		ReturnType:       returnType,
		IsReturnNullable: isReturnNullable,
	}, nil
}

func (fp *FuncParser) parseParameter(paramStr string) (Parameter, error) {
	parts := strings.Split(paramStr, "=")
	typePart := strings.TrimSpace(parts[0])

	param := Parameter{HasDefault: len(parts) > 1}

	if param.HasDefault {
		param.DefaultValue = fp.sanitizeDefaultValue(strings.TrimSpace(parts[1]))
	}

	matches := typeNameRegex.FindStringSubmatch(typePart)

	if len(matches) < 3 {
		return Parameter{}, fmt.Errorf("invalid parameter format: %s", paramStr)
	}

	typeStr := strings.TrimSpace(matches[1])
	param.Name = strings.TrimSpace(matches[2])
	param.IsNullable = strings.HasPrefix(typeStr, "?")
	param.Type = strings.TrimPrefix(typeStr, "?")

	return param, nil
}

func (fp *FuncParser) sanitizeDefaultValue(value string) string {
	if strings.HasPrefix(value, "[") && strings.HasSuffix(value, "]") {
		return value
	}
	if strings.ToLower(value) == "null" {
		return "null"
	}

	return strings.Trim(value, "'\"")
}
