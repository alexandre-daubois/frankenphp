package types

/*
Garder seulement string, slices, maps, et peut-être objects
*/

/*
#include <stdlib.h>
#include <string.h>
#include "types.h"
*/
import "C"
import (
	"fmt"
	"runtime"
	"unsafe"
)

const (
	Z_STR = iota
	Z_INTEGER
	Z_FLOAT
	Z_BOOL
	Z_ARR
	Z_NULL
	Z_OBJ
)

type PHPValue struct {
	CValue *C.go_value
}

func (v *PHPValue) finalize() {
	if v.CValue != nil {
		cleanupGoValue(v.CValue)
		v.CValue = nil
	}
}

func newPHPValue(cValue *C.go_value) *PHPValue {
	if cValue == nil {
		return nil
	}
	v := &PHPValue{CValue: cValue}
	runtime.SetFinalizer(v, (*PHPValue).finalize)
	return v
}

// EXPERIMENTAL
func String(s string) *PHPValue {
	cstr := C.CString(s)
	result := (*C.go_value)(C.malloc(C.sizeof_go_value))
	if result == nil {
		C.free(unsafe.Pointer(cstr))
		return nil
	}
	result.value_type = Z_STR
	result.str_len = C.int(len(s))
	*(**C.char)(unsafe.Pointer(&result.data)) = cstr

	return newPHPValue(result)
}

// EXPERIMENTAL
func Int(i int64) *PHPValue {
	result := (*C.go_value)(C.malloc(C.sizeof_go_value))
	if result == nil {
		return nil
	}
	result.value_type = Z_INTEGER
	*(*C.long)(unsafe.Pointer(&result.data)) = C.long(i)

	return newPHPValue(result)
}

// EXPERIMENTAL
func Float(f float64) *PHPValue {
	result := (*C.go_value)(C.malloc(C.sizeof_go_value))
	if result == nil {
		return nil
	}
	result.value_type = Z_FLOAT
	*(*C.double)(unsafe.Pointer(&result.data)) = C.double(f)

	return newPHPValue(result)
}

// EXPERIMENTAL
func Bool(b bool) *PHPValue {
	result := (*C.go_value)(C.malloc(C.sizeof_go_value))
	if result == nil {
		return nil
	}
	result.value_type = Z_BOOL

	boolPtr := (*C.int)(unsafe.Pointer(&result.data))
	if b {
		*boolPtr = 1
	} else {
		*boolPtr = 0
	}

	return newPHPValue(result)
}

// EXPERIMENTAL
// TODO nil doit fonctionner, inutile?
func Null() *PHPValue {
	result := (*C.go_value)(C.malloc(C.sizeof_go_value))
	if result == nil {
		return nil
	}
	result.value_type = Z_NULL
	result.str_len = 0

	return newPHPValue(result)
}

// EXPERIMENTAL
func Array(slice []interface{}) *PHPValue {
	result := (*C.go_value)(C.malloc(C.sizeof_go_value))
	if result == nil {
		return nil
	}
	result.value_type = Z_ARR

	length := len(slice)
	arr := (*C.go_array)(C.malloc(C.sizeof_go_array))
	if arr == nil {
		C.free(unsafe.Pointer(result))
		return nil
	}
	arr.length = C.int(length)
	arr.is_associative = 0 // indexed array

	if length > 0 {
		elementsSize := C.size_t(length) * C.size_t(unsafe.Sizeof(uintptr(0)))
		arr.elements = (**C.go_array_element)(C.malloc(elementsSize))
		if arr.elements == nil {
			C.free(unsafe.Pointer(arr))
			C.free(unsafe.Pointer(result))
			return nil
		}

		elementsSlice := (*[1 << 30]*C.go_array_element)(unsafe.Pointer(arr.elements))[:length:length]

		for i, v := range slice {
			elem := (*C.go_array_element)(C.malloc(C.sizeof_go_array_element))
			if elem == nil {
				// Clean up previously allocated elements
				for j := 0; j < i; j++ {
					if elementsSlice[j] != nil {
						cleanupGoArrayElement(elementsSlice[j])
					}
				}
				C.free(unsafe.Pointer(arr.elements))
				C.free(unsafe.Pointer(arr))
				C.free(unsafe.Pointer(result))
				return nil
			}
			elem.key = nil
			elem.index = C.int(i)
			elem.value = convertInterfaceToPHPValue(v).CValue

			elementsSlice[i] = elem
		}
	} else {
		arr.elements = nil
	}

	*(**C.go_array)(unsafe.Pointer(&result.data)) = arr
	return newPHPValue(result)
}

// EXPERIMENTAL
func Map(m map[string]interface{}) *PHPValue {
	result := (*C.go_value)(C.malloc(C.sizeof_go_value))
	if result == nil {
		return nil
	}
	result.value_type = Z_ARR

	length := len(m)
	arr := (*C.go_array)(C.malloc(C.sizeof_go_array))
	if arr == nil {
		C.free(unsafe.Pointer(result))
		return nil
	}
	arr.length = C.int(length)
	arr.is_associative = 1 // associative array

	if length > 0 {
		elementsSize := C.size_t(length) * C.size_t(unsafe.Sizeof(uintptr(0)))
		arr.elements = (**C.go_array_element)(C.malloc(elementsSize))
		if arr.elements == nil {
			C.free(unsafe.Pointer(arr))
			C.free(unsafe.Pointer(result))
			return nil
		}

		elementsSlice := (*[1 << 30]*C.go_array_element)(unsafe.Pointer(arr.elements))[:length:length]

		i := 0
		for k, v := range m {
			elem := (*C.go_array_element)(C.malloc(C.sizeof_go_array_element))
			if elem == nil {
				// Clean up previously allocated elements
				for j := 0; j < i; j++ {
					if elementsSlice[j] != nil {
						cleanupGoArrayElement(elementsSlice[j])
					}
				}
				C.free(unsafe.Pointer(arr.elements))
				C.free(unsafe.Pointer(arr))
				C.free(unsafe.Pointer(result))
				return nil
			}
			elem.key = C.CString(k)
			elem.index = -1 // string key
			elem.value = convertInterfaceToPHPValue(v).CValue

			elementsSlice[i] = elem
			i++
		}
	} else {
		arr.elements = nil
	}

	*(**C.go_array)(unsafe.Pointer(&result.data)) = arr
	return newPHPValue(result)
}

// EXPERIMENTAL
func Object(properties map[string]interface{}, className string) *PHPValue {
	result := (*C.go_value)(C.malloc(C.sizeof_go_value))
	if result == nil {
		return nil
	}
	result.value_type = Z_OBJ

	length := len(properties)
	obj := (*C.go_object)(C.malloc(C.sizeof_go_object))
	if obj == nil {
		C.free(unsafe.Pointer(result))
		return nil
	}

	if className != "" {
		obj.class_name = C.CString(className)
	} else {
		obj.class_name = C.CString("stdClass")
	}

	obj.property_count = C.int(length)

	if length > 0 {
		propertiesSize := C.size_t(length) * C.size_t(unsafe.Sizeof(uintptr(0)))
		obj.properties = (**C.go_object_property)(C.malloc(propertiesSize))
		if obj.properties == nil {
			C.free(unsafe.Pointer(obj.class_name))
			C.free(unsafe.Pointer(obj))
			C.free(unsafe.Pointer(result))
			return nil
		}

		propertiesSlice := (*[1 << 30]*C.go_object_property)(unsafe.Pointer(obj.properties))[:length:length]

		i := 0
		for name, value := range properties {
			prop := (*C.go_object_property)(C.malloc(C.sizeof_go_object_property))
			if prop == nil {
				// Clean up previously allocated properties
				for j := 0; j < i; j++ {
					if propertiesSlice[j] != nil {
						cleanupGoObjectProperty(propertiesSlice[j])
					}
				}
				C.free(unsafe.Pointer(obj.properties))
				C.free(unsafe.Pointer(obj.class_name))
				C.free(unsafe.Pointer(obj))
				C.free(unsafe.Pointer(result))
				return nil
			}
			prop.name = C.CString(name)
			prop.value = convertInterfaceToPHPValue(value).CValue

			propertiesSlice[i] = prop
			i++
		}
	} else {
		obj.properties = nil
	}

	*(**C.go_object)(unsafe.Pointer(&result.data)) = obj
	return newPHPValue(result)
}

func convertInterfaceToPHPValue(v interface{}) *PHPValue {
	switch val := v.(type) {
	case nil:
		return Null()
	case string:
		return String(val)
	case int:
		return Int(int64(val))
	case int64:
		return Int(val)
	case float64:
		return Float(val)
	case bool:
		return Bool(val)
	case []interface{}:
		return Array(val)
	case map[string]interface{}:
		return Map(val)
	default:
		return String(fmt.Sprintf("%v", val))
	}
}

// EXPERIMENTAL
func CStringToGoString(gs *C.go_string) string {
	if gs == nil || gs.data == nil {
		return ""
	}
	return C.GoStringN(gs.data, gs.length)
}

// EXPERIMENTAL
func PhpArrayToSlice(arr *C.go_array) []interface{} {
	if arr == nil || arr.length == 0 {
		return []interface{}{}
	}

	length := int(arr.length)
	result := make([]interface{}, length)

	elementsSlice := (*[1 << 30]*C.go_array_element)(unsafe.Pointer(arr.elements))[:length:length]

	for i := 0; i < length; i++ {
		elem := elementsSlice[i]
		if elem == nil || elem.value == nil {
			result[i] = nil
			continue
		}

		result[i] = convertGoValueToInterface(elem.value)
	}

	return result
}

// EXPERIMENTAL
func PhpArrayToMap(arr *C.go_array) map[string]interface{} {
	if arr == nil || arr.length == 0 {
		return map[string]interface{}{}
	}

	length := int(arr.length)
	result := make(map[string]interface{})

	elementsSlice := (*[1 << 30]*C.go_array_element)(unsafe.Pointer(arr.elements))[:length:length]

	for i := 0; i < length; i++ {
		elem := elementsSlice[i]
		if elem == nil || elem.value == nil {
			continue
		}

		var key string
		if elem.key != nil {
			key = C.GoString(elem.key)
		} else {
			key = fmt.Sprintf("%d", elem.index)
		}

		result[key] = convertGoValueToInterface(elem.value)
	}

	return result
}

// EXPERIMENTAL
func PhpObjectToMap(obj *C.go_object) map[string]interface{} {
	if obj == nil || obj.property_count == 0 {
		return map[string]interface{}{}
	}

	length := int(obj.property_count)
	result := make(map[string]interface{})

	propertiesSlice := (*[1 << 30]*C.go_object_property)(unsafe.Pointer(obj.properties))[:length:length]

	for i := 0; i < length; i++ {
		prop := propertiesSlice[i]
		if prop == nil || prop.value == nil || prop.name == nil {
			continue
		}

		key := C.GoString(prop.name)
		result[key] = convertGoValueToInterface(prop.value)
	}

	return result
}

func convertGoValueToInterface(val *C.go_value) interface{} {
	if val == nil {
		return nil
	}

	switch val.value_type {
	case 0: // string
		strPtr := *(**C.char)(unsafe.Pointer(&val.data))
		return C.GoStringN(strPtr, C.int(val.str_len))
	case 1: // integer
		intPtr := (*C.long)(unsafe.Pointer(&val.data))
		return int64(*intPtr)
	case 2: // float
		floatPtr := (*C.double)(unsafe.Pointer(&val.data))
		return float64(*floatPtr)
	case 3: // bool
		boolPtr := (*C.int)(unsafe.Pointer(&val.data))
		return *boolPtr != 0
	case 4: // array
		arrayPtr := *(**C.go_array)(unsafe.Pointer(&val.data))
		if IsAssociativeArray(arrayPtr) {
			return PhpArrayToMap(arrayPtr)
		}
		return PhpArrayToSlice(arrayPtr)
	case 5: // null
		return nil
	case 6: // object
		objectPtr := *(**C.go_object)(unsafe.Pointer(&val.data))
		return PhpObjectToMap(objectPtr)
	default:
		return nil
	}
}

// EXPERIMENTAL
func IsAssociativeArray(arr *C.go_array) bool {
	if arr == nil {
		return false
	}
	return int(arr.is_associative) == 1
}

// EXPERIMENTAL
func NullableString(n *C.go_nullable) *string {
	if n == nil || n.is_null != 0 || n.value == nil {
		return nil
	}
	if n.value_type != Z_STR {
		return nil
	}

	goStr := (*C.go_string)(n.value)
	result := C.GoStringN(goStr.data, C.int(goStr.length))
	return &result
}

// EXPERIMENTAL
func NullableInt(n *C.go_nullable) *int64 {
	if n == nil || n.is_null != 0 || n.value == nil {
		return nil
	}
	if n.value_type != Z_INTEGER {
		return nil
	}

	longPtr := (*C.long)(n.value)
	result := int64(*longPtr)
	return &result
}

// EXPERIMENTAL
func NullableFloat(n *C.go_nullable) *float64 {
	if n == nil || n.is_null != 0 || n.value == nil {
		return nil
	}
	if n.value_type != Z_FLOAT {
		return nil
	}

	doublePtr := (*C.double)(n.value)
	result := float64(*doublePtr)
	return &result
}

// EXPERIMENTAL
func NullableBool(n *C.go_nullable) *bool {
	if n == nil || n.is_null != 0 || n.value == nil {
		return nil
	}
	if n.value_type != Z_BOOL {
		return nil
	}

	intPtr := (*C.int)(n.value)
	result := *intPtr != 0
	return &result
}

// EXPERIMENTAL
func NullableArray(n *C.go_nullable) []interface{} {
	if n == nil || n.is_null != 0 || n.value == nil {
		return nil
	}
	if n.value_type != Z_ARR {
		return nil
	}

	arr := (*C.go_array)(n.value)
	return PhpArrayToSlice(arr)
}

// EXPERIMENTAL
func NullableMap(n *C.go_nullable) map[string]interface{} {
	if n == nil || n.is_null != 0 || n.value == nil {
		return nil
	}
	if n.value_type != Z_ARR {
		return nil
	}

	arr := (*C.go_array)(n.value)
	return PhpArrayToMap(arr)
}

// EXPERIMENTAL
func NullableObject(n *C.go_nullable) map[string]interface{} {
	if n == nil || n.is_null != 0 || n.value == nil {
		return nil
	}
	if n.value_type != Z_OBJ {
		return nil
	}

	obj := (*C.go_object)(n.value)
	return PhpObjectToMap(obj)
}

func cleanupGoValue(v *C.go_value) {
	if v != nil {
		if v.value_type == Z_STR && (*(**C.char)(unsafe.Pointer(&v.data))) != nil {
			C.free(unsafe.Pointer(*(**C.char)(unsafe.Pointer(&v.data))))
		} else if v.value_type == Z_ARR && (*(**C.go_array)(unsafe.Pointer(&v.data))) != nil {
			cleanupGoArray(*(**C.go_array)(unsafe.Pointer(&v.data)))
		} else if v.value_type == Z_OBJ && (*(**C.go_object)(unsafe.Pointer(&v.data))) != nil {
			cleanupGoObject(*(**C.go_object)(unsafe.Pointer(&v.data)))
		}
		C.free(unsafe.Pointer(v))
	}
}

func cleanupGoArray(arr *C.go_array) {
	if arr != nil {
		if arr.elements != nil {
			elementsSlice := (*[1 << 30]*C.go_array_element)(unsafe.Pointer(arr.elements))[:arr.length:arr.length]
			for i := 0; i < int(arr.length); i++ {
				if elementsSlice[i] != nil {
					cleanupGoArrayElement(elementsSlice[i])
				}
			}
			C.free(unsafe.Pointer(arr.elements))
		}
		C.free(unsafe.Pointer(arr))
	}
}

func cleanupGoArrayElement(elem *C.go_array_element) {
	if elem != nil {
		if elem.key != nil {
			C.free(unsafe.Pointer(elem.key))
		}
		if elem.value != nil {
			cleanupGoValue(elem.value)
		}
		C.free(unsafe.Pointer(elem))
	}
}

func cleanupGoObject(obj *C.go_object) {
	if obj != nil {
		if obj.class_name != nil {
			C.free(unsafe.Pointer(obj.class_name))
		}
		if obj.properties != nil {
			propertiesSlice := (*[1 << 30]*C.go_object_property)(unsafe.Pointer(obj.properties))[:obj.property_count:obj.property_count]
			for i := 0; i < int(obj.property_count); i++ {
				if propertiesSlice[i] != nil {
					cleanupGoObjectProperty(propertiesSlice[i])
				}
			}
			C.free(unsafe.Pointer(obj.properties))
		}
		C.free(unsafe.Pointer(obj))
	}
}

func cleanupGoObjectProperty(prop *C.go_object_property) {
	if prop != nil {
		if prop.name != nil {
			C.free(unsafe.Pointer(prop.name))
		}
		if prop.value != nil {
			cleanupGoValue(prop.value)
		}
		C.free(unsafe.Pointer(prop))
	}
}
