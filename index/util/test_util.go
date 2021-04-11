package util

import "reflect"

func IntSliceEquals(a, b interface{}) bool {
	ta, tb := reflect.TypeOf(a).Kind(), reflect.TypeOf(b).Kind()
	av, bv := reflect.ValueOf(a), reflect.ValueOf(b)
	if ta != tb {
		return false
	}
	switch ta {
	case reflect.Slice, reflect.Array:
		if av.Len() != bv.Len() {
			return false
		}
		for i := 0; i < av.Len(); i++ {
			if !IntSliceEquals(av.Index(i).Interface(), bv.Index(i).Interface()) {
				return false
			}
		}
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		if av.Int() != bv.Int() {
			return false
		}
	default:
		panic("unknown type:" + ta.String())
	}
	return true
}
