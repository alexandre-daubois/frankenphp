package extgen

import (
	"fmt"
	"strings"
)

type PHPFuncGenerator struct{}

func (pfg *PHPFuncGenerator) generate(fn PHPFunction) string {
	var builder strings.Builder

	paramInfo := pfg.analyzeParameters(fn.Params)

	builder.WriteString(fmt.Sprintf("PHP_FUNCTION(%s)\n{\n", fn.Name))

	if decl := pfg.generateParamDeclarations(fn.Params); decl != "" {
		builder.WriteString(decl + "\n")
	}

	builder.WriteString(pfg.generateParamParsing(fn.Params, paramInfo.RequiredCount) + "\n")

	builder.WriteString(pfg.generateGoCall(fn) + "\n")

	if returnCode := pfg.generateReturnCode(fn.ReturnType); returnCode != "" {
		builder.WriteString(returnCode + "\n")
	}

	builder.WriteString("}\n\n")

	return builder.String()
}

type ParameterInfo struct {
	RequiredCount int
	TotalCount    int
}

func (pfg *PHPFuncGenerator) analyzeParameters(params []Parameter) ParameterInfo {
	info := ParameterInfo{TotalCount: len(params)}

	for _, param := range params {
		if !param.HasDefault {
			info.RequiredCount++
		}
	}

	return info
}

func (pfg *PHPFuncGenerator) generateParamDeclarations(params []Parameter) string {
	if len(params) == 0 {
		return ""
	}

	var declarations []string

	for _, param := range params {
		declarations = append(declarations, pfg.generateSingleParamDeclaration(param)...)
	}

	return "    " + strings.Join(declarations, "\n    ")
}

func (pfg *PHPFuncGenerator) generateSingleParamDeclaration(param Parameter) []string {
	var decls []string

	switch param.Type {
	case "string":
		decls = append(decls, fmt.Sprintf("zend_string *%s = NULL;", param.Name))
	case "int":
		defaultVal := pfg.getDefaultValue(param, "0")
		decls = append(decls, fmt.Sprintf("zend_long %s = %s;", param.Name, defaultVal))
	case "float":
		defaultVal := pfg.getDefaultValue(param, "0.0")
		decls = append(decls, fmt.Sprintf("double %s = %s;", param.Name, defaultVal))
	case "bool":
		defaultVal := pfg.getDefaultValue(param, "0")
		if param.HasDefault && param.DefaultValue == "true" {
			defaultVal = "1"
		}

		decls = append(decls, fmt.Sprintf("zend_bool %s = %s;", param.Name, defaultVal))
	}

	return decls
}

func (pfg *PHPFuncGenerator) getDefaultValue(param Parameter, fallback string) string {
	if !param.HasDefault || param.DefaultValue == "" {
		return fallback
	}
	return param.DefaultValue
}

func (pfg *PHPFuncGenerator) generateParamParsing(params []Parameter, requiredCount int) string {
	if len(params) == 0 {
		return `
	if (zend_parse_parameters_none() == FAILURE) {
		RETURN_THROWS();
	}`
	}

	var builder strings.Builder
	builder.WriteString(fmt.Sprintf("    ZEND_PARSE_PARAMETERS_START(%d, %d)", requiredCount, len(params)))

	optionalStarted := false
	for _, param := range params {
		if param.HasDefault && !optionalStarted {
			builder.WriteString("\n        Z_PARAM_OPTIONAL")
			optionalStarted = true
		}

		builder.WriteString(pfg.generateParamParsingMacro(param))
	}

	builder.WriteString("\n    ZEND_PARSE_PARAMETERS_END();")
	return builder.String()
}

func (pfg *PHPFuncGenerator) generateParamParsingMacro(param Parameter) string {
	switch param.Type {
	case "string":
		return fmt.Sprintf("\n        Z_PARAM_STR(%s)", param.Name)
	case "int":
		return fmt.Sprintf("\n        Z_PARAM_LONG(%s)", param.Name)
	case "float":
		return fmt.Sprintf("\n        Z_PARAM_DOUBLE(%s)", param.Name)
	case "bool":
		return fmt.Sprintf("\n        Z_PARAM_BOOL(%s)", param.Name)
	default:
		return ""
	}
}

func (pfg *PHPFuncGenerator) generateGoCall(fn PHPFunction) string {
	callParams := pfg.generateGoCallParams(fn.Params)

	if fn.ReturnType == "void" {
		return fmt.Sprintf("    %s(%s);", fn.Name, callParams)
	}

	if fn.ReturnType == "string" {
		return fmt.Sprintf("    zend_string *result = %s(%s);", fn.Name, callParams)
	}

	return fmt.Sprintf("    %s result = %s(%s);", pfg.getCReturnType(fn.ReturnType), fn.Name, callParams)
}

func (pfg *PHPFuncGenerator) generateGoCallParams(params []Parameter) string {
	if len(params) == 0 {
		return ""
	}

	var goParams []string
	for _, param := range params {
		goParams = append(goParams, pfg.generateSingleGoCallParam(param))
	}

	return strings.Join(goParams, ", ")
}

func (pfg *PHPFuncGenerator) generateSingleGoCallParam(param Parameter) string {
	switch param.Type {
	case "string":
		return param.Name
	case "int":
		return fmt.Sprintf("(long) %s", param.Name)
	case "float":
		return fmt.Sprintf("(double) %s", param.Name)
	case "bool":
		return fmt.Sprintf("(int) %s", param.Name)
	default:
		return param.Name
	}
}

func (pfg *PHPFuncGenerator) getCReturnType(returnType string) string {
	switch returnType {
	case "string":
		return "zend_string*"
	case "int":
		return "long"
	case "float":
		return "double"
	case "bool":
		return "int"
	default:
		return "void"
	}
}

func (pfg *PHPFuncGenerator) generateReturnCode(returnType string) string {
	switch returnType {
	case "string":
		return `
	if (result) {
        RETURN_STR(result);
    } else {
        RETURN_EMPTY_STRING();
    }`
	case "int":
		return `    RETURN_LONG(result);`
	case "float":
		return `    RETURN_DOUBLE(result);`
	case "bool":
		return `    RETURN_BOOL(result);`
	default:
		return ""
	}
}
