#ifndef TYPES_H
#define TYPES_H

typedef struct go_value go_value;
typedef struct go_array go_array;
typedef struct go_array_element go_array_element;
typedef struct go_object go_object;
typedef struct go_object_property go_object_property;

typedef struct {
    int length;
    char* data;
} go_string;

typedef struct {
    void* value;     // Pointer to the real value
    int is_null;     // 1 if null, 0 otherwise
    int value_type;  // Value type (0=string, 1=int, 2=float, 3=bool, 4=array, 5=null, 6=object)
} go_nullable;

struct go_array_element {
    char* key;      // NULL for numeric keys
    int index;      // -1 for string keys
    go_value* value;
};

struct go_array {
    int length;
    go_array_element** elements;
    int is_associative; // 1 if it contains string keys, 0 if numeric
};

struct go_object_property {
    char* name;      // Property name
    go_value* value; // Property value
};

struct go_object {
    char* class_name; // can be NULL for stdClass
    int property_count;
    go_object_property** properties;
};

typedef union {
    char* str_val;
    long int_val;
    double float_val;
    int bool_val;
    go_array* array_val;
    go_object* object_val;
} go_value_data;

struct go_value {
    int value_type;  // 0=string, 1=int, 2=float, 3=bool, 4=array, 5=null, 6=object
    int str_len;
    go_value_data data;
};

#endif