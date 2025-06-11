#include <php.h>
#include <Zend/zend_API.h>
#include <stddef.h>

#include "{{.BaseName}}.h"
#include "{{.BaseName}}_arginfo.h"
#include "_cgo_export.h"

static int (*original_php_register_internal_extensions_func)(void) = NULL;
{{- if .Classes}}

static zend_object_handlers object_handlers_{{.BaseName}};

typedef struct {
    uintptr_t go_handle;
    char* class_name;
    zend_object std; /* This MUST be the last struct field to memory alignement problems */
} {{.BaseName}}_object;

static inline {{.BaseName}}_object *{{.BaseName}}_object_from_obj(zend_object *obj) {
    return ({{.BaseName}}_object*)((char*)(obj) - offsetof({{.BaseName}}_object, std));
}

static zend_object *{{.BaseName}}_create_object(zend_class_entry *ce) {
    {{.BaseName}}_object *intern = ecalloc(1, sizeof({{.BaseName}}_object) + zend_object_properties_size(ce));
    
    zend_object_std_init(&intern->std, ce);
    object_properties_init(&intern->std, ce);
    
    intern->std.handlers = &object_handlers_{{.BaseName}};
    intern->go_handle = 0; /* will be set in __construct */
    intern->class_name = estrdup(ZSTR_VAL(ce->name));
    
    return &intern->std;
}

static void {{.BaseName}}_free_object(zend_object *object) {
    {{.BaseName}}_object *intern = {{.BaseName}}_object_from_obj(object);
    
    if (intern->class_name) {
        efree(intern->class_name);
    }
    
    if (intern->go_handle != 0) {
        removeGoObject(intern->go_handle);
    }
    
    zend_object_std_dtor(&intern->std);
}


static zend_function *{{.BaseName}}_get_method(zend_object **object, zend_string *method, const zval *key) {
    {{.BaseName}}_object *intern = {{.BaseName}}_object_from_obj(*object);
    
    {{- range .Classes}}{{range .Methods}}
    if (strcmp(intern->class_name, "{{.ClassName}}") == 0 && 
        strcmp(ZSTR_VAL(method), "{{.PHPName}}") == 0) {
        return zend_std_get_method(object, method, key);
    }
    {{- end}}{{end}}
    
    return zend_std_get_method(object, method, key);
}

void init_object_handlers() {
    memcpy(&object_handlers_{{.BaseName}}, &std_object_handlers, sizeof(zend_object_handlers));
    object_handlers_{{.BaseName}}.get_method = {{.BaseName}}_get_method;
    object_handlers_{{.BaseName}}.free_obj = {{.BaseName}}_free_object;
    object_handlers_{{.BaseName}}.offset = offsetof({{.BaseName}}_object, std);
}
{{- end}}

{{- range .Classes}}
static zend_class_entry *{{.Name}}_ce = NULL;

PHP_METHOD({{.Name}}, __construct) {
    if (zend_parse_parameters_none() == FAILURE) {
        RETURN_THROWS();
    }
    
    {{$.BaseName}}_object *intern = {{$.BaseName}}_object_from_obj(Z_OBJ_P(ZEND_THIS));
    
    intern->go_handle = create_{{.GoStruct}}_object();
}

{{- range .Methods}}

PHP_METHOD({{.ClassName}}, {{.PHPName}}) {
    {{$.BaseName}}_object *intern = {{$.BaseName}}_object_from_obj(Z_OBJ_P(ZEND_THIS));
    
    if (intern->go_handle == 0) {
        zend_throw_error(NULL, "Go object not found in registry");
        RETURN_THROWS();
    }
    
    {{if .Params}}
    {{range $i, $param := .Params}}
    {{if eq $param.Type "string"}}zend_string *{{$param.Name}} = NULL;{{if $param.IsNullable}}
    zend_bool {{$param.Name}}_is_null = 0;{{end}}{{end}}
    {{if eq $param.Type "int"}}zend_long {{$param.Name}} = {{if $param.HasDefault}}{{$param.DefaultValue}}{{else}}0{{end}};{{if $param.IsNullable}}
    zend_bool {{$param.Name}}_is_null = 0;{{end}}{{end}}
    {{if eq $param.Type "float"}}double {{$param.Name}} = {{if $param.HasDefault}}{{$param.DefaultValue}}{{else}}0.0{{end}};{{if $param.IsNullable}}
    zend_bool {{$param.Name}}_is_null = 0;{{end}}{{end}}
    {{if eq $param.Type "bool"}}zend_bool {{$param.Name}} = {{if $param.HasDefault}}{{if eq $param.DefaultValue "true"}}1{{else}}0{{end}}{{else}}0{{end}};{{if $param.IsNullable}}
    zend_bool {{$param.Name}}_is_null = 0;{{end}}{{end}}
    {{end}}
    
    {{$requiredCount := 0}}{{range .Params}}{{if not .HasDefault}}{{$requiredCount = inc $requiredCount}}{{end}}{{end}}
    ZEND_PARSE_PARAMETERS_START({{$requiredCount}}, {{len .Params}})
        {{$optionalStarted := false}}{{range .Params}}{{if .HasDefault}}{{if not $optionalStarted}}Z_PARAM_OPTIONAL
        {{$optionalStarted = true}}{{end}}{{end}}{{if .IsNullable}}{{if eq .Type "string"}}Z_PARAM_STR_OR_NULL({{.Name}}, {{.Name}}_is_null){{else if eq .Type "int"}}Z_PARAM_LONG_OR_NULL({{.Name}}, {{.Name}}_is_null){{else if eq .Type "float"}}Z_PARAM_DOUBLE_OR_NULL({{.Name}}, {{.Name}}_is_null){{else if eq .Type "bool"}}Z_PARAM_BOOL_OR_NULL({{.Name}}, {{.Name}}_is_null){{end}}{{else}}{{if eq .Type "string"}}Z_PARAM_STR({{.Name}}){{else if eq .Type "int"}}Z_PARAM_LONG({{.Name}}){{else if eq .Type "float"}}Z_PARAM_DOUBLE({{.Name}}){{else if eq .Type "bool"}}Z_PARAM_BOOL({{.Name}}){{end}}{{end}}
        {{end}}ZEND_PARSE_PARAMETERS_END();
    {{else}}
    if (zend_parse_parameters_none() == FAILURE) {
        RETURN_THROWS();
    }
    {{end}}
    
    {{- if ne .ReturnType "void"}}
    {{- if eq .ReturnType "string"}}
    zend_string* result = {{.Name}}_wrapper(intern->go_handle{{if .Params}}{{range .Params}}, {{if .IsNullable}}{{if eq .Type "string"}}{{.Name}}_is_null ? NULL : {{.Name}}{{else if eq .Type "int"}}{{.Name}}_is_null ? NULL : &{{.Name}}{{else if eq .Type "float"}}{{.Name}}_is_null ? NULL : &{{.Name}}{{else if eq .Type "bool"}}{{.Name}}_is_null ? NULL : &{{.Name}}{{end}}{{else}}{{.Name}}{{end}}{{end}}{{end}});
    RETURN_STR(result);
    {{- else if eq .ReturnType "int"}}
    zend_long result = {{.Name}}_wrapper(intern->go_handle{{if .Params}}{{range .Params}}, {{if .IsNullable}}{{if eq .Type "string"}}{{.Name}}_is_null ? NULL : {{.Name}}{{else if eq .Type "int"}}{{.Name}}_is_null ? NULL : &{{.Name}}{{else if eq .Type "float"}}{{.Name}}_is_null ? NULL : &{{.Name}}{{else if eq .Type "bool"}}{{.Name}}_is_null ? NULL : &{{.Name}}{{end}}{{else}}(long){{.Name}}{{end}}{{end}}{{end}});
    RETURN_LONG(result);
    {{- else if eq .ReturnType "float"}}
    double result = {{.Name}}_wrapper(intern->go_handle{{if .Params}}{{range .Params}}, {{if .IsNullable}}{{if eq .Type "string"}}{{.Name}}_is_null ? NULL : {{.Name}}{{else if eq .Type "int"}}{{.Name}}_is_null ? NULL : &{{.Name}}{{else if eq .Type "float"}}{{.Name}}_is_null ? NULL : &{{.Name}}{{else if eq .Type "bool"}}{{.Name}}_is_null ? NULL : &{{.Name}}{{end}}{{else}}(double){{.Name}}{{end}}{{end}}{{end}});
    RETURN_DOUBLE(result);
    {{- else if eq .ReturnType "bool"}}
    int result = {{.Name}}_wrapper(intern->go_handle{{if .Params}}{{range .Params}}, {{if .IsNullable}}{{if eq .Type "string"}}{{.Name}}_is_null ? NULL : {{.Name}}{{else if eq .Type "int"}}{{.Name}}_is_null ? NULL : &{{.Name}}{{else if eq .Type "float"}}{{.Name}}_is_null ? NULL : &{{.Name}}{{else if eq .Type "bool"}}{{.Name}}_is_null ? NULL : &{{.Name}}{{end}}{{else}}(int){{.Name}}{{end}}{{end}}{{end}});
    RETURN_BOOL(result);
    {{- end}}
    {{- else}}
    {{.Name}}_wrapper(intern->go_handle{{if .Params}}{{range .Params}}, {{if .IsNullable}}{{if eq .Type "string"}}{{.Name}}_is_null ? NULL : {{.Name}}{{else if eq .Type "int"}}{{.Name}}_is_null ? NULL : &{{.Name}}{{else if eq .Type "float"}}{{.Name}}_is_null ? NULL : &{{.Name}}{{else if eq .Type "bool"}}{{.Name}}_is_null ? NULL : &{{.Name}}{{end}}{{else}}{{if eq .Type "string"}}{{.Name}}{{else if eq .Type "int"}}(long){{.Name}}{{else if eq .Type "float"}}(double){{.Name}}{{else if eq .Type "bool"}}(int){{.Name}}{{end}}{{end}}{{end}}{{end}});
    {{- end}}
}
{{end}}
{{end}}

{{- if .Classes}}

void register_all_classes() {
    init_object_handlers();
    
    {{- range .Classes}}
    {{.Name}}_ce = register_class_{{.Name}}();
    if (!{{.Name}}_ce) {
        php_error_docref(NULL, E_ERROR, "Failed to register class {{.Name}}");
        return;
    }
    {{.Name}}_ce->create_object = {{$.BaseName}}_create_object;
    
    /* Register class constants for {{.Name}} */
    {{- $className := .Name}}
    {{- range $.Constants}}
    {{- if eq .ClassName $className}}
    {{- if .IsIota}}zend_declare_class_constant_long({{.ClassName}}_ce, "{{.Name}}", sizeof("{{.Name}}")-1, {{.Name}});
    {{- else if eq .Type "string"}}zend_declare_class_constant_string({{.ClassName}}_ce, "{{.Name}}", sizeof("{{.Name}}")-1, {{.CValue}});
    {{- else if eq .Type "bool"}}zend_declare_class_constant_long({{.ClassName}}_ce, "{{.Name}}", sizeof("{{.Name}}")-1, {{if eq .Value "true"}}1{{else}}0{{end}});
    {{- else if eq .Type "float"}}zend_declare_class_constant_double({{.ClassName}}_ce, "{{.Name}}", sizeof("{{.Name}}")-1, {{.CValue}});
    {{- else}}zend_declare_class_constant_long({{.ClassName}}_ce, "{{.Name}}", sizeof("{{.Name}}")-1, {{.CValue}});
    {{- end}}
    {{- end}}
    {{- end}}
    {{- end}}
}
{{- end}}

PHP_MINIT_FUNCTION({{.BaseName}}) {
    {{ if .Classes}}register_all_classes();{{end}}
    
    {{ range .Constants}}
    {{- if eq .ClassName ""}}
    {{ if .IsIota}}REGISTER_LONG_CONSTANT("{{.Name}}", {{.Name}}, CONST_CS | CONST_PERSISTENT);
    {{ else if eq .Type "string"}}REGISTER_STRING_CONSTANT("{{.Name}}", {{.CValue}}, CONST_CS | CONST_PERSISTENT);
    {{ else if eq .Type "bool"}}REGISTER_LONG_CONSTANT("{{.Name}}", {{if eq .Value "true"}}1{{else}}0{{end}}, CONST_CS | CONST_PERSISTENT);
    {{ else if eq .Type "float"}}REGISTER_DOUBLE_CONSTANT("{{.Name}}", {{.CValue}}, CONST_CS | CONST_PERSISTENT);
    {{ else}}REGISTER_LONG_CONSTANT("{{.Name}}", {{.CValue}}, CONST_CS | CONST_PERSISTENT);
    {{- end}}
    {{- end}}
    {{- end}}

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
