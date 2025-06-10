#include <php.h>
#include <Zend/zend_API.h>
#include <stddef.h>

#include "{{.BaseName}}.h"
#include "{{.BaseName}}_arginfo.h"
#include "_cgo_export.h"

static int (*original_php_register_internal_extensions_func)(void) = NULL;

{{if .Classes}}
/* Object structure for class instances */
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

static zval *{{.BaseName}}_read_property(zend_object *object, zend_string *member, int type, void **cache_slot, zval *rv) {
    {{.BaseName}}_object *intern = {{.BaseName}}_object_from_obj(object);
    
    if (intern->go_handle == 0) {
        return zend_std_read_property(object, member, type, cache_slot, rv);
    }
    
    const char *prop_name = ZSTR_VAL(member);
    
    {{range $class := .Classes}}
    if (strcmp(intern->class_name, "{{$class.Name}}") == 0) {
        {{range $class.Properties}}
        if (strcmp(prop_name, "{{.Name | ToLower}}") == 0) {
            {{if eq .Type "string"}}
            zend_string* result = get_{{$class.Name}}_{{.Name}}_property(intern->go_handle);
            ZVAL_STR(rv, result);
            return rv;
            {{else if eq .Type "int"}}
            zend_long result = get_{{$class.Name}}_{{.Name}}_property(intern->go_handle);
            ZVAL_LONG(rv, result);
            return rv;
            {{else if eq .Type "float"}}
            float result = get_{{$class.Name}}_{{.Name}}_property(intern->go_handle);
            ZVAL_DOUBLE(rv, result);
            return rv;
            {{else if eq .Type "bool"}}
            bool result = get_{{$class.Name}}_{{.Name}}_property(intern->go_handle);
            ZVAL_BOOL(rv, result);
            return rv;
            {{end}}
        }
        {{end}}
    }
    {{end}}
    
    return zend_std_read_property(object, member, type, cache_slot, rv);
}

static zval *{{.BaseName}}_write_property(zend_object *object, zend_string *member, zval *value, void **cache_slot) {
    {{.BaseName}}_object *intern = {{.BaseName}}_object_from_obj(object);
    
    if (intern->go_handle == 0) {
        return zend_std_write_property(object, member, value, cache_slot);
    }
    
    const char *prop_name = ZSTR_VAL(member);
    
    {{range $class := .Classes}}
    if (strcmp(intern->class_name, "{{$class.Name}}") == 0) {
        {{range $class.Properties}}
        if (strcmp(prop_name, "{{.Name | ToLower}}") == 0) {
            {{if eq .Type "string"}}
            if (Z_TYPE_P(value) == IS_STRING) {
                set_{{$class.Name}}_{{.Name}}_property(intern->go_handle, Z_STR_P(value));
            }
            {{else if eq .Type "int"}}
            if (Z_TYPE_P(value) == IS_LONG) {
                set_{{$class.Name}}_{{.Name}}_property(intern->go_handle, value);
            }
            {{else if eq .Type "float"}}
            if (Z_TYPE_P(value) == IS_DOUBLE) {
                set_{{$class.Name}}_{{.Name}}_property(intern->go_handle, Z_DVAL_P(value));
            }
            {{else if eq .Type "bool"}}
            if (Z_TYPE_P(value) == IS_TRUE || Z_TYPE_P(value) == IS_FALSE) {
                set_{{$class.Name}}_{{.Name}}_property(intern->go_handle, (zend_long)(Z_TYPE_P(value) == IS_TRUE ? 1 : 0));
            }
            {{end}}
            return value;
        }
        {{end}}
    }
    {{end}}
    
    return zend_std_write_property(object, member, value, cache_slot);
}

static HashTable *{{.BaseName}}_get_properties(zend_object *object) {
    {{.BaseName}}_object *intern = {{.BaseName}}_object_from_obj(object);
    HashTable *props = zend_std_get_properties(object);
    
    if (intern->go_handle == 0) {
        return props;
    }
    
    {{range $class := .Classes}}
    if (strcmp(intern->class_name, "{{$class.Name}}") == 0) {
        {{range $class.Properties}}
        zval property_value_{{.Name}};
        {{if eq .Type "string"}}
        zend_string* result_{{.Name}} = get_{{$class.Name}}_{{.Name}}_property(intern->go_handle);
        ZVAL_STR(&property_value_{{.Name}}, result_{{.Name}});
        {{else if eq .Type "int"}}
        zend_long result_{{.Name}} = get_{{$class.Name}}_{{.Name}}_property(intern->go_handle);
        ZVAL_LONG(&property_value_{{.Name}}, result_{{.Name}});
        {{else if eq .Type "float"}}
        float result_{{.Name}} = get_{{$class.Name}}_{{.Name}}_property(intern->go_handle);
        ZVAL_DOUBLE(&property_value_{{.Name}}, result_{{.Name}});
        {{else if eq .Type "bool"}}
        bool result_{{.Name}} = get_{{$class.Name}}_{{.Name}}_property(intern->go_handle);
        ZVAL_BOOL(&property_value_{{.Name}}, result_{{.Name}});
        {{end}}
        zend_hash_str_update(props, "{{.Name | ToLower}}", sizeof("{{.Name | ToLower}}") - 1, &property_value_{{.Name}});
        {{end}}
    }
    {{end}}
    
    return props;
}

static zend_function *{{.BaseName}}_get_method(zend_object **object, zend_string *method, const zval *key) {
    {{.BaseName}}_object *intern = {{.BaseName}}_object_from_obj(*object);
    
    {{range .Classes}}{{range .Methods}}
    if (strcmp(intern->class_name, "{{.ClassName}}") == 0 && 
        strcmp(ZSTR_VAL(method), "{{.PHPName}}") == 0) {
        /* handled by generated method wrapper */
        return zend_std_get_method(object, method, key);
    }{{end}}{{end}}
    
    return zend_std_get_method(object, method, key);
}

void init_object_handlers() {
    memcpy(&object_handlers_{{.BaseName}}, &std_object_handlers, sizeof(zend_object_handlers));
    object_handlers_{{.BaseName}}.get_method = {{.BaseName}}_get_method;
    object_handlers_{{.BaseName}}.read_property = {{.BaseName}}_read_property;
    object_handlers_{{.BaseName}}.write_property = {{.BaseName}}_write_property;
    object_handlers_{{.BaseName}}.get_properties = {{.BaseName}}_get_properties;
    object_handlers_{{.BaseName}}.free_obj = {{.BaseName}}_free_object;
    object_handlers_{{.BaseName}}.offset = offsetof({{.BaseName}}_object, std);
}
{{end}}

{{range .Classes}}static zend_class_entry *{{.Name}}_ce = NULL;

PHP_METHOD({{.Name}}, __construct) {
    if (zend_parse_parameters_none() == FAILURE) {
        RETURN_THROWS();
    }
    
    {{$.BaseName}}_object *intern = {{$.BaseName}}_object_from_obj(Z_OBJ_P(ZEND_THIS));
    
    intern->go_handle = create_{{.GoStruct}}_object();
}

{{range .Methods}}
PHP_METHOD({{.ClassName}}, {{.PHPName}}) {
    {{$.BaseName}}_object *intern = {{$.BaseName}}_object_from_obj(Z_OBJ_P(ZEND_THIS));
    
    if (intern->go_handle == 0) {
        zend_throw_error(NULL, "Go object not found in registry");
        RETURN_THROWS();
    }
    
    {{if .Params}}
    {{range $i, $param := .Params}}
    {{if eq $param.Type "string"}}zend_string *{{$param.Name}}_zstr;{{end}}
    {{if eq $param.Type "int"}}zend_long {{$param.Name}}_long;{{end}}
    {{if eq $param.Type "float"}}double {{$param.Name}}_double;{{end}}
    {{if eq $param.Type "bool"}}zend_bool {{$param.Name}}_bool;{{end}}
    {{end}}
    
    if (zend_parse_parameters(ZEND_NUM_ARGS(), "{{range .Params}}{{if eq .Type "string"}}S{{else if eq .Type "int"}}l{{else if eq .Type "float"}}d{{else if eq .Type "bool"}}b{{end}}{{end}}"{{range .Params}}, {{if eq .Type "string"}}&{{.Name}}_zstr{{else if eq .Type "int"}}&{{.Name}}_long{{else if eq .Type "float"}}&{{.Name}}_double{{else if eq .Type "bool"}}&{{.Name}}_bool{{end}}{{end}}) == FAILURE) {
        RETURN_THROWS();
    }
    {{end}}
    
    {{if ne .ReturnType "void"}}
    {{if eq .ReturnType "string"}}
    zend_string* result = {{.Name}}_wrapper(intern->go_handle{{if .Params}}{{range .Params}}, {{if eq .Type "string"}}{{.Name}}_zstr{{else if eq .Type "int"}}(zend_long){{.Name}}_long{{else if eq .Type "float"}}(float){{.Name}}_double{{else if eq .Type "bool"}}(bool){{.Name}}_bool{{end}}{{end}}{{end}});
    RETURN_STR(result);
    {{else if eq .ReturnType "int"}}
    zend_long result = {{.Name}}_wrapper(intern->go_handle{{if .Params}}{{range .Params}}, {{if eq .Type "string"}}{{.Name}}_zstr{{else if eq .Type "int"}}(zend_long){{.Name}}_long{{else if eq .Type "float"}}(float){{.Name}}_double{{else if eq .Type "bool"}}(bool){{.Name}}_bool{{end}}{{end}}{{end}});
    RETURN_LONG(result);
    {{else if eq .ReturnType "float"}}
    float result = {{.Name}}_wrapper(intern->go_handle{{if .Params}}{{range .Params}}, {{if eq .Type "string"}}{{.Name}}_zstr{{else if eq .Type "int"}}(zend_long){{.Name}}_long{{else if eq .Type "float"}}(float){{.Name}}_double{{else if eq .Type "bool"}}(bool){{.Name}}_bool{{end}}{{end}}{{end}});
    RETURN_DOUBLE(result);
    {{else if eq .ReturnType "bool"}}
    bool result = {{.Name}}_wrapper(intern->go_handle{{if .Params}}{{range .Params}}, {{if eq .Type "string"}}{{.Name}}_zstr{{else if eq .Type "int"}}(zend_long){{.Name}}_long{{else if eq .Type "float"}}(float){{.Name}}_double{{else if eq .Type "bool"}}(bool){{.Name}}_bool{{end}}{{end}}{{end}});
    RETURN_BOOL(result);
    {{end}}
    {{else}}
    {{.Name}}_wrapper(intern->go_handle{{if .Params}}{{range .Params}}, {{if eq .Type "string"}}{{.Name}}_zstr{{else if eq .Type "int"}}(zend_long){{.Name}}_long{{else if eq .Type "float"}}(float){{.Name}}_double{{else if eq .Type "bool"}}(bool){{.Name}}_bool{{end}}{{end}}{{end}});
    {{end}}
}
{{end}}
{{end}}

{{if .Classes}}
void register_all_classes() {
    init_object_handlers();
    
    {{range .Classes}}{{.Name}}_ce = register_class_{{.Name}}();
    if (!{{.Name}}_ce) {
        php_error_docref(NULL, E_ERROR, "Failed to register class {{.Name}}");
        return;
    }
    {{.Name}}_ce->create_object = {{$.BaseName}}_create_object;
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
