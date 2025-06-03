package extgen

import (
	"strings"
	"testing"
)

func TestPHPFunctionGenerator_Generate(t *testing.T) {
	tests := []struct {
		name     string
		function PHPFunction
		contains []string // Strings that should be present in the output
	}{
		{
			name: "simple string function",
			function: PHPFunction{
				Name:       "greet",
				ReturnType: "string",
				Params: []Parameter{
					{Name: "name", Type: "string"},
				},
			},
			contains: []string{
				"PHP_FUNCTION(greet)",
				"char *name = NULL;",
				"size_t name_len = 0;",
				"Z_PARAM_STRING(name, name_len)",
				"go_value *result = greet(",
				"RETVAL_STRINGL(result->data.str_val, result->str_len)",
			},
		},
		{
			name: "function with nullable parameter",
			function: PHPFunction{
				Name:       "process",
				ReturnType: "int",
				Params: []Parameter{
					{Name: "data", Type: "string", IsNullable: true},
				},
			},
			contains: []string{
				"PHP_FUNCTION(process)",
				"zend_bool data_is_null = 0;",
				"Z_PARAM_STRING_OR_NULL(data, data_len)",
				"go_nullable* data_nullable_ptr = NULL;",
				"create_nullable_string(data, data_len, data_is_null)",
			},
		},
		{
			name: "function with default parameter",
			function: PHPFunction{
				Name:       "calculate",
				ReturnType: "int",
				Params: []Parameter{
					{Name: "base", Type: "int"},
					{Name: "multiplier", Type: "int", HasDefault: true, DefaultValue: "2"},
				},
			},
			contains: []string{
				"PHP_FUNCTION(calculate)",
				"zend_long base = 0;",
				"zend_long multiplier = 2;",
				"ZEND_PARSE_PARAMETERS_START(1, 2)",
				"Z_PARAM_OPTIONAL",
				"Z_PARAM_LONG(base)",
				"Z_PARAM_LONG(multiplier)",
			},
		},
		{
			name: "void function",
			function: PHPFunction{
				Name:       "doSomething",
				ReturnType: "void",
				Params: []Parameter{
					{Name: "action", Type: "string"},
				},
			},
			contains: []string{
				"PHP_FUNCTION(doSomething)",
				"doSomething(",
			},
		},
		{
			name: "function with array parameter",
			function: PHPFunction{
				Name:       "processArray",
				ReturnType: "array",
				Params: []Parameter{
					{Name: "items", Type: "array"},
				},
			},
			contains: []string{
				"PHP_FUNCTION(processArray)",
				"zval *items = NULL;",
				"Z_PARAM_ARRAY(items)",
				"zval_to_go_array(items)",
				"go_array_to_zval(result->data.array_val, return_value)",
			},
		},
	}

	generator := PHPFuncGenerator{}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := generator.generate(tt.function)

			for _, expected := range tt.contains {
				if !strings.Contains(result, expected) {
					t.Errorf("Generated code should contain '%s'\nGenerated:\n%s", expected, result)
				}
			}

			if !strings.HasPrefix(result, "PHP_FUNCTION(") {
				t.Errorf("Generated code should start with PHP_FUNCTION")
			}

			if !strings.HasSuffix(strings.TrimSpace(result), "}") {
				t.Errorf("Generated code should end with closing brace")
			}
		})
	}
}

func TestPHPFunctionGenerator_GenerateParamDeclarations(t *testing.T) {
	tests := []struct {
		name     string
		params   []Parameter
		contains []string
	}{
		{
			name: "string parameter",
			params: []Parameter{
				{Name: "message", Type: "string"},
			},
			contains: []string{
				"char *message = NULL;",
				"size_t message_len = 0;",
			},
		},
		{
			name: "nullable int parameter",
			params: []Parameter{
				{Name: "count", Type: "int", IsNullable: true},
			},
			contains: []string{
				"zend_long count = 0;",
				"zval *count_zval = NULL;",
				"zend_bool count_is_null = 0;",
			},
		},
		{
			name: "bool with default",
			params: []Parameter{
				{Name: "enabled", Type: "bool", HasDefault: true, DefaultValue: "true"},
			},
			contains: []string{
				"zend_bool enabled = 1;",
			},
		},
	}

	generator := PHPFuncGenerator{}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := generator.generateParamDeclarations(tt.params)

			for _, expected := range tt.contains {
				if !strings.Contains(result, expected) {
					t.Errorf("types.Parameter declarations should contain '%s'\nGenerated:\n%s", expected, result)
				}
			}
		})
	}
}

func TestPHPFunctionGenerator_GenerateReturnCode(t *testing.T) {
	tests := []struct {
		name       string
		returnType string
		nullable   bool
		contains   []string
	}{
		{
			name:       "string return",
			returnType: "string",
			nullable:   false,
			contains: []string{
				"RETVAL_STRINGL(result->data.str_val, result->str_len)",
				"RETVAL_EMPTY_STRING()",
				"cleanup_go_value(result)",
			},
		},
		{
			name:       "nullable string return",
			returnType: "string",
			nullable:   true,
			contains: []string{
				"RETVAL_STRINGL(result->data.str_val, result->str_len)",
				"RETVAL_NULL()",
				"cleanup_go_value(result)",
			},
		},
		{
			name:       "int return",
			returnType: "int",
			nullable:   false,
			contains: []string{
				"RETVAL_LONG(result->data.int_val)",
				"RETVAL_LONG(0)",
			},
		},
		{
			name:       "bool return",
			returnType: "bool",
			nullable:   false,
			contains: []string{
				"RETVAL_BOOL(result->data.bool_val)",
				"RETVAL_FALSE",
			},
		},
		{
			name:       "array return",
			returnType: "array",
			nullable:   false,
			contains: []string{
				"go_array_to_zval(result->data.array_val, return_value)",
				"array_init(return_value)",
			},
		},
	}

	generator := PHPFuncGenerator{}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := generator.generateReturnCode(tt.returnType, tt.nullable)

			for _, expected := range tt.contains {
				if !strings.Contains(result, expected) {
					t.Errorf("Return code should contain '%s'\nGenerated:\n%s", expected, result)
				}
			}
		})
	}
}

func TestPHPFunctionGenerator_GenerateGoCallParams(t *testing.T) {
	tests := []struct {
		name     string
		params   []Parameter
		expected string
	}{
		{
			name:     "no parameters",
			params:   []Parameter{},
			expected: "",
		},
		{
			name: "simple string parameter",
			params: []Parameter{
				{Name: "message", Type: "string"},
			},
			expected: "&(go_string){message_len, message}",
		},
		{
			name: "nullable parameter",
			params: []Parameter{
				{Name: "data", Type: "string", IsNullable: true},
			},
			expected: "data_nullable_ptr",
		},
		{
			name: "multiple parameters",
			params: []Parameter{
				{Name: "name", Type: "string"},
				{Name: "age", Type: "int"},
			},
			expected: "&(go_string){name_len, name}, (long)age",
		},
	}

	generator := PHPFuncGenerator{}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := generator.generateGoCallParams(tt.params)

			if result != tt.expected {
				t.Errorf("generateGoCallParams() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestPHPFunctionGenerator_AnalyzeParameters(t *testing.T) {
	tests := []struct {
		name          string
		params        []Parameter
		expectedReq   int
		expectedTotal int
	}{
		{
			name:          "no parameters",
			params:        []Parameter{},
			expectedReq:   0,
			expectedTotal: 0,
		},
		{
			name: "all required",
			params: []Parameter{
				{Name: "a", Type: "string"},
				{Name: "b", Type: "int"},
			},
			expectedReq:   2,
			expectedTotal: 2,
		},
		{
			name: "mixed required and optional",
			params: []Parameter{
				{Name: "required", Type: "string"},
				{Name: "optional", Type: "int", HasDefault: true, DefaultValue: "10"},
			},
			expectedReq:   1,
			expectedTotal: 2,
		},
		{
			name: "all optional",
			params: []Parameter{
				{Name: "opt1", Type: "string", HasDefault: true, DefaultValue: "hello"},
				{Name: "opt2", Type: "int", HasDefault: true, DefaultValue: "0"},
			},
			expectedReq:   0,
			expectedTotal: 2,
		},
	}

	generator := PHPFuncGenerator{}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			info := generator.analyzeParameters(tt.params)

			if info.RequiredCount != tt.expectedReq {
				t.Errorf("analyzeParameters() RequiredCount = %v, want %v", info.RequiredCount, tt.expectedReq)
			}

			if info.TotalCount != tt.expectedTotal {
				t.Errorf("analyzeParameters() TotalCount = %v, want %v", info.TotalCount, tt.expectedTotal)
			}
		})
	}
}
