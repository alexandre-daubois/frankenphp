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

	if handling := pfg.generateDefaultHandling(fn.Params); handling != "" {
		builder.WriteString(handling + "\n")
	}

	if setup := pfg.generateNullableSetup(fn.Params); setup != "" {
		builder.WriteString(setup + "\n")
	}

	builder.WriteString(pfg.generateGoCall(fn) + "\n")

	if returnCode := pfg.generateReturnCode(fn.ReturnType, fn.IsReturnNullable); returnCode != "" {
		builder.WriteString(returnCode + "\n")
	}

	if cleanup := pfg.generateCleanup(fn.Params); cleanup != "" {
		builder.WriteString(cleanup + "\n")
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
		decls = append(decls, fmt.Sprintf("char *%s = NULL;", param.Name))
		decls = append(decls, fmt.Sprintf("size_t %s_len = 0;", param.Name))
		if param.IsNullable {
			decls = append(decls, fmt.Sprintf("zend_bool %s_is_null = 0;", param.Name))
		}

	case "int":
		defaultVal := pfg.getDefaultValue(param, "0")
		decls = append(decls, fmt.Sprintf("zend_long %s = %s;", param.Name, defaultVal))
		if param.IsNullable {
			decls = append(decls, fmt.Sprintf("zval *%s_zval = NULL;", param.Name))
			decls = append(decls, fmt.Sprintf("zend_bool %s_is_null = 0;", param.Name))
		}

	case "float":
		defaultVal := pfg.getDefaultValue(param, "0.0")
		decls = append(decls, fmt.Sprintf("double %s = %s;", param.Name, defaultVal))
		if param.IsNullable {
			decls = append(decls, fmt.Sprintf("zval *%s_zval = NULL;", param.Name))
			decls = append(decls, fmt.Sprintf("zend_bool %s_is_null = 0;", param.Name))
		}

	case "bool":
		defaultVal := pfg.getDefaultValue(param, "0")
		if param.HasDefault && param.DefaultValue == "true" {
			defaultVal = "1"
		}
		decls = append(decls, fmt.Sprintf("zend_bool %s = %s;", param.Name, defaultVal))
		if param.IsNullable {
			decls = append(decls, fmt.Sprintf("zval *%s_zval = NULL;", param.Name))
			decls = append(decls, fmt.Sprintf("zend_bool %s_is_null = 0;", param.Name))
		}

	case "array", "object":
		decls = append(decls, fmt.Sprintf("zval *%s = NULL;", param.Name))
		if param.HasDefault || param.IsNullable {
			decls = append(decls, fmt.Sprintf("zend_bool %s_is_null = 1;", param.Name))
		}
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
		return "    if (zend_parse_parameters_none() == FAILURE) {\n        RETURN_THROWS();\n    }"
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
		if param.IsNullable {
			return fmt.Sprintf("\n        Z_PARAM_STRING_OR_NULL(%s, %s_len)", param.Name, param.Name)
		}
		return fmt.Sprintf("\n        Z_PARAM_STRING(%s, %s_len)", param.Name, param.Name)

	case "int":
		if param.IsNullable {
			return fmt.Sprintf("\n        Z_PARAM_ZVAL_OR_NULL(%s_zval)", param.Name)
		}
		return fmt.Sprintf("\n        Z_PARAM_LONG(%s)", param.Name)

	case "float":
		if param.IsNullable {
			return fmt.Sprintf("\n        Z_PARAM_ZVAL_OR_NULL(%s_zval)", param.Name)
		}
		return fmt.Sprintf("\n        Z_PARAM_DOUBLE(%s)", param.Name)

	case "bool":
		if param.IsNullable {
			return fmt.Sprintf("\n        Z_PARAM_ZVAL_OR_NULL(%s_zval)", param.Name)
		}
		return fmt.Sprintf("\n        Z_PARAM_BOOL(%s)", param.Name)

	case "array":
		if param.IsNullable {
			return fmt.Sprintf("\n        Z_PARAM_ARRAY_OR_NULL(%s)", param.Name)
		}
		return fmt.Sprintf("\n        Z_PARAM_ARRAY(%s)", param.Name)

	case "object":
		if param.IsNullable {
			return fmt.Sprintf("\n        Z_PARAM_OBJECT_OR_NULL(%s)", param.Name)
		}
		return fmt.Sprintf("\n        Z_PARAM_OBJECT(%s)", param.Name)

	default:
		return ""
	}
}

func (pfg *PHPFuncGenerator) generateDefaultHandling(params []Parameter) string {
	var handling []string

	for _, param := range params {
		if param.IsNullable {
			if h := pfg.generateNullableHandling(param); h != "" {
				handling = append(handling, h)
			}
		}

		if param.Type == "array" && param.HasDefault {
			if h := pfg.generateArrayDefaultHandling(param); h != "" {
				handling = append(handling, h)
			}
		}
	}

	if len(handling) > 0 {
		return "    " + strings.Join(handling, "\n    ")
	}
	return ""
}

func (pfg *PHPFuncGenerator) generateNullableHandling(param Parameter) string {
	switch param.Type {
	case "string":
		return fmt.Sprintf("%s_is_null = (%s == NULL) ? 1 : 0;", param.Name, param.Name)

	case "int":
		return fmt.Sprintf(`if (%s_zval == NULL || Z_TYPE_P(%s_zval) == IS_NULL) {
        %s_is_null = 1;
    } else {
        %s_is_null = 0;
        %s = Z_LVAL_P(%s_zval);
    }`, param.Name, param.Name, param.Name, param.Name, param.Name, param.Name)

	case "float":
		return fmt.Sprintf(`if (%s_zval == NULL || Z_TYPE_P(%s_zval) == IS_NULL) {
        %s_is_null = 1;
    } else {
        %s_is_null = 0;
        %s = Z_DVAL_P(%s_zval);
    }`, param.Name, param.Name, param.Name, param.Name, param.Name, param.Name)

	case "bool":
		return fmt.Sprintf(`if (%s_zval == NULL || Z_TYPE_P(%s_zval) == IS_NULL) {
        %s_is_null = 1;
    } else {
        %s_is_null = 0;
        %s = Z_TYPE_P(%s_zval) == IS_TRUE ? 1 : 0;
    }`, param.Name, param.Name, param.Name, param.Name, param.Name, param.Name)

	case "array", "object":
		return fmt.Sprintf("%s_is_null = (%s == NULL) ? 1 : 0;", param.Name, param.Name)

	default:
		return ""
	}
}

func (pfg *PHPFuncGenerator) generateArrayDefaultHandling(param Parameter) string {
	if param.DefaultValue == "null" || param.DefaultValue == "NULL" {
		if !param.IsNullable {
			return fmt.Sprintf(`if (%s == NULL) {
        %s_is_null = 1;
    } else {
        %s_is_null = 0;
    }`, param.Name, param.Name, param.Name)
		}
		return ""
	}

	var builder strings.Builder
	builder.WriteString(fmt.Sprintf(`if (%s == NULL) {
        zval default_%s;
        array_init(&default_%s);
        %s = &default_%s;
        %s_is_null = 0;`, param.Name, param.Name, param.Name, param.Name, param.Name, param.Name))

	if strings.HasPrefix(param.DefaultValue, "[") && strings.HasSuffix(param.DefaultValue, "]") {
		arrayContent := strings.Trim(param.DefaultValue, "[]")
		if arrayContent != "" {
			elements := strings.Split(arrayContent, ",")
			for i, elem := range elements {
				elem = strings.TrimSpace(strings.Trim(elem, "'\""))
				builder.WriteString(fmt.Sprintf(`
        add_index_string(&default_%s, %d, "%s");`, param.Name, i, elem))
			}
		}
	}

	builder.WriteString(fmt.Sprintf(`
    } else {
        %s_is_null = 0;
    }`, param.Name))

	return builder.String()
}

func (pfg *PHPFuncGenerator) generateNullableSetup(params []Parameter) string {
	var setup []string

	for _, param := range params {
		if param.IsNullable || (param.Type == "array" && param.HasDefault) || (param.Type == "object" && param.HasDefault) {
			setup = append(setup, fmt.Sprintf("go_nullable* %s_nullable_ptr = NULL;", param.Name))
		}
	}

	for _, param := range params {
		if nullable := pfg.generateNullableCreation(param); nullable != "" {
			setup = append(setup, nullable)
		}
	}

	if len(setup) > 0 {
		return "    " + strings.Join(setup, "\n    ")
	}
	return ""
}

func (pfg *PHPFuncGenerator) generateNullableCreation(param Parameter) string {
	if !param.IsNullable && !(param.Type == "array" && param.HasDefault) && !(param.Type == "object" && param.HasDefault) {
		return ""
	}

	switch param.Type {
	case "string":
		return fmt.Sprintf("%s_nullable_ptr = create_nullable_string(%s, %s_len, %s_is_null);",
			param.Name, param.Name, param.Name, param.Name)
	case "int":
		return fmt.Sprintf("%s_nullable_ptr = create_nullable_long(%s, %s_is_null);",
			param.Name, param.Name, param.Name)
	case "float":
		return fmt.Sprintf("%s_nullable_ptr = create_nullable_double(%s, %s_is_null);",
			param.Name, param.Name, param.Name)
	case "bool":
		return fmt.Sprintf("%s_nullable_ptr = create_nullable_bool(%s, %s_is_null);",
			param.Name, param.Name, param.Name)
	case "array":
		return fmt.Sprintf("%s_nullable_ptr = create_nullable_array(%s, %s_is_null);",
			param.Name, param.Name, param.Name)
	case "object":
		return fmt.Sprintf("%s_nullable_ptr = create_nullable_object(%s, %s_is_null);",
			param.Name, param.Name, param.Name)
	default:
		return ""
	}
}

func (pfg *PHPFuncGenerator) generateGoCall(fn PHPFunction) string {
	callParams := pfg.generateGoCallParams(fn.Params)

	if fn.ReturnType == "void" {
		return fmt.Sprintf("    %s(%s);", fn.Name, callParams)
	}
	return fmt.Sprintf("    go_value *result = %s(%s);", fn.Name, callParams)
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
		if param.IsNullable {
			return fmt.Sprintf("%s_nullable_ptr", param.Name)
		}
		if param.HasDefault {
			defaultValue := strings.Trim(param.DefaultValue, "'\"")
			return fmt.Sprintf("(%s ? &(go_string){%s_len, %s} : &(go_string){%d, \"%s\"})",
				param.Name, param.Name, param.Name, len(defaultValue), defaultValue)
		}
		return fmt.Sprintf("&(go_string){%s_len, %s}", param.Name, param.Name)

	case "int":
		if param.IsNullable {
			return fmt.Sprintf("%s_nullable_ptr", param.Name)
		}
		return fmt.Sprintf("(long)%s", param.Name)

	case "float":
		if param.IsNullable {
			return fmt.Sprintf("%s_nullable_ptr", param.Name)
		}
		return fmt.Sprintf("(double)%s", param.Name)

	case "bool":
		if param.IsNullable {
			return fmt.Sprintf("%s_nullable_ptr", param.Name)
		}
		return fmt.Sprintf("(int)%s", param.Name)

	case "array":
		if param.IsNullable || param.HasDefault {
			return fmt.Sprintf("%s_nullable_ptr", param.Name)
		}
		return fmt.Sprintf("zval_to_go_array(%s)", param.Name)

	case "object":
		if param.IsNullable || param.HasDefault {
			return fmt.Sprintf("%s_nullable_ptr", param.Name)
		}
		return fmt.Sprintf("zval_to_go_object(%s)", param.Name)

	default:
		return param.Name
	}
}

func (pfg *PHPFuncGenerator) generateReturnCode(returnType string, isNullable bool) string {
	switch returnType {
	case "void":
		return ""

	case "string":
		if isNullable {
			return `    if (result && result->value_type == 0) {
        RETVAL_STRINGL(result->data.str_val, result->str_len);
    } else {
        RETVAL_NULL();
    }
    if (result) cleanup_go_value(result);`
		}
		return `    if (result && result->value_type == 0) {
        RETVAL_STRINGL(result->data.str_val, result->str_len);
    } else {
        RETVAL_EMPTY_STRING();
    }
    if (result) cleanup_go_value(result);`

	case "int":
		if isNullable {
			return `    if (result && result->value_type == 1) {
        RETVAL_LONG(result->data.int_val);
    } else {
        RETVAL_NULL();
    }
    if (result) cleanup_go_value(result);`
		}
		return `    if (result && result->value_type == 1) {
        RETVAL_LONG(result->data.int_val);
    } else {
        RETVAL_LONG(0);
    }
    if (result) cleanup_go_value(result);`

	case "float":
		if isNullable {
			return `    if (result && result->value_type == 2) {
        RETVAL_DOUBLE(result->data.float_val);
    } else {
        RETVAL_NULL();
    }
    if (result) cleanup_go_value(result);`
		}
		return `    if (result && result->value_type == 2) {
        RETVAL_DOUBLE(result->data.float_val);
    } else {
        RETVAL_DOUBLE(0.0);
    }
    if (result) cleanup_go_value(result);`

	case "bool":
		return `    if (result && result->value_type == 3) {
        RETVAL_BOOL(result->data.bool_val);
    } else {
        RETVAL_FALSE;
    }
    if (result) cleanup_go_value(result);`

	case "array":
		if isNullable {
			return `    if (result && result->value_type == 4 && result->data.array_val) {
        go_array_to_zval(result->data.array_val, return_value);
    } else {
        RETVAL_NULL();
    }
    if (result) cleanup_go_value(result);`
		}
		return `    if (result && result->value_type == 4 && result->data.array_val) {
        go_array_to_zval(result->data.array_val, return_value);
    } else {
        array_init(return_value);
    }
    if (result) cleanup_go_value(result);`

	case "object":
		if isNullable {
			return `    if (result && result->value_type == 6 && result->data.object_val) {
        go_object_to_zval(result->data.object_val, return_value);
    } else {
        RETVAL_NULL();
    }
    if (result) cleanup_go_value(result);`
		}
		return `    if (result && result->value_type == 6 && result->data.object_val) {
        go_object_to_zval(result->data.object_val, return_value);
    } else {
        object_init(return_value);
    }
    if (result) cleanup_go_value(result);`

	default:
		return ""
	}
}

func (pfg *PHPFuncGenerator) generateCleanup(params []Parameter) string {
	var cleanup []string

	for _, param := range params {
		if param.IsNullable || (param.Type == "array" && param.HasDefault) || (param.Type == "object" && param.HasDefault) {
			cleanup = append(cleanup, fmt.Sprintf("if (%s_nullable_ptr) cleanup_go_nullable(%s_nullable_ptr);", param.Name, param.Name))
		}
	}

	if len(cleanup) > 0 {
		return "    " + strings.Join(cleanup, "\n    ")
	}
	return ""
}
