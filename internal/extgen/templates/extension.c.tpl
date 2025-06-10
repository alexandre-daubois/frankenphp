#include <php.h>
#include <Zend/zend_API.h>

#include "_cgo_export.h"
#include "{{.BaseName}}.h"
#include "{{.BaseName}}_arginfo.h"

static int (*original_php_register_internal_extensions_func)(void) = NULL;

{{range .Classes}}static zend_class_entry *{{.Name}}_ce = NULL;

PHP_METHOD({{.Name}}, __construct) {
    if (zend_parse_parameters_none() == FAILURE) {
        RETURN_THROWS();
    }
}
{{end}}

{{if .Classes}}
void register_all_classes() {
    {{range .Classes}}{{.Name}}_ce = register_class_{{.Name}}();
    if (!{{.Name}}_ce) {
        php_error_docref(NULL, E_ERROR, "Failed to register class {{.Name}}");
        return;
    }
{{end}}
}{{end}}

PHP_MINIT_FUNCTION({{.BaseName}}) {
    {{if .Classes}}register_all_classes();{{end}}
    
    {{range .Constants}}{{if .IsIota}}REGISTER_LONG_CONSTANT("{{.Name}}", {{.Name}}, CONST_CS | CONST_PERSISTENT);
    {{else if eq .Type "string"}}REGISTER_STRING_CONSTANT("{{.Name}}", {{.CValue}}, CONST_CS | CONST_PERSISTENT);
    {{else if eq .Type "bool"}}REGISTER_LONG_CONSTANT("{{.Name}}", {{if eq .Value "true"}}1{{else}}0{{end}}, CONST_CS | CONST_PERSISTENT);
    {{else if eq .Type "float"}}REGISTER_DOUBLE_CONSTANT("{{.Name}}", {{.CValue}}, CONST_CS | CONST_PERSISTENT);
    {{else}}REGISTER_LONG_CONSTANT("{{.Name}}", {{.CValue}}, CONST_CS | CONST_PERSISTENT);
    {{end}}{{end}}

    return SUCCESS;
}

zend_module_entry {{.BaseName}}_module_entry = {STANDARD_MODULE_HEADER,
                                         "{{.BaseName}}",
                                         ext_functions,  /* Functions */
                                         PHP_MINIT({{.BaseName}}),  /* MINIT */
                                         NULL,           /* MSHUTDOWN */
                                         NULL,           /* RINIT */
                                         NULL,           /* RSHUTDOWN */
                                         NULL,           /* MINFO */
                                         "{{.Version}}", // version
                                         STANDARD_MODULE_PROPERTIES};

PHPAPI int register_internal_extensions(void) {
  if (original_php_register_internal_extensions_func != NULL &&
      original_php_register_internal_extensions_func() != SUCCESS) {
    return FAILURE;
  }

  zend_module_entry *module = &{{.BaseName}}_module_entry;
  if (zend_register_internal_module(module) == NULL) {
    return FAILURE;
  };

  return SUCCESS;
}

void register_extension() {
  original_php_register_internal_extensions_func =
      php_register_internal_extensions_func;
  php_register_internal_extensions_func = register_internal_extensions;
}
