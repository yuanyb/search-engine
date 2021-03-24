package util

import "strconv"

// 解析失败，则不设置值，因此使用指针的方式修改，如果返回值的话，使用起来就比较繁琐了
func ToBool(dest *bool, value string) {
	if b, err := strconv.ParseBool(value); err == nil {
		*dest = b
	}
}

func ToInt(dest *int, value string) {
	if i, err := strconv.ParseInt(value, 10, 32); err == nil {
		*dest = int(i)
	}
}

func ToInt64(dest *int64, value string) {
	if i, err := strconv.ParseInt(value, 10, 64); err == nil {
		*dest = i
	}
}
