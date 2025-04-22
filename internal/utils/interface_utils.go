package utils

import "reflect"

// IsNil checks if an interface value is nil
func IsNil(i interface{}) bool {
	if i == nil {
		return true
	}
	
	// Check if the underlying value is nil
	v := reflect.ValueOf(i)
	kind := v.Kind()
	
	return (kind == reflect.Ptr ||
		kind == reflect.Interface ||
		kind == reflect.Slice ||
		kind == reflect.Map ||
		kind == reflect.Chan ||
		kind == reflect.Func) &&
		v.IsNil()
}
