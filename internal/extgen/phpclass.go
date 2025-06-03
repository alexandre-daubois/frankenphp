package extgen

import (
	"fmt"
	"strings"
)

type PHPClassGenerator struct{}

func (pcg *PHPClassGenerator) generate(classes []PHPClass) string {
	var builder strings.Builder

	if len(classes) == 0 {
		builder.WriteString("void register_all_classes() {\n    // No classes to register\n}\n\n")
		return builder.String()
	}

	for _, class := range classes {
		className := SanitizePackageName(strings.ToLower(class.Name))
		builder.WriteString(fmt.Sprintf("static zend_class_entry *%s_ce = NULL;\n", className))
	}
	builder.WriteString("\n")

	for _, class := range classes {
		builder.WriteString(fmt.Sprintf(`PHP_METHOD(%s, __construct) {
    if (zend_parse_parameters_none() == FAILURE) {
        RETURN_THROWS();
    }
}

`, class.Name))
	}

	builder.WriteString("void register_all_classes() {\n")
	for _, class := range classes {
		className := SanitizePackageName(strings.ToLower(class.Name))
		builder.WriteString(fmt.Sprintf("    %s_ce = register_class_%s();\n", className, class.Name))
		builder.WriteString(fmt.Sprintf("    if (!%s_ce) {\n", className))
		builder.WriteString(fmt.Sprintf("        php_error_docref(NULL, E_ERROR, \"Failed to register class %s\");\n", class.Name))
		builder.WriteString("        return;\n")
		builder.WriteString("    }\n")
	}
	builder.WriteString("}\n\n")

	return builder.String()
}
