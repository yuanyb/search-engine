// 分词
package core

import "log"

// 这种方式的问题是，会将其他乱七八糟的字符都建索引
//var ignoreCharsSet = make(map[rune]struct{})
//
//func init() {
//	ignoreChars := " \f\n\r\t\v!@#$%^&*()_+-=[]{}\\|;:'\",.<>/?`——【】；：‘“、？。《　》，！·"
//	for _, ch := range ignoreChars {
//		ignoreCharsSet[ch] = struct{}{}
//	}
//}

// 只为下列字符建立索引
func isIgnoredChar(char rune) bool {
	if (char >= 0x4E00 && char <= 0x9FA5) || // 汉字
		(char >= 'A' && char <= 'Z') || // A-Z
		(char >= 'a' && char <= 'z') || // a-z
		(char >= '0' && char <= '9') { // 0-9
		return false
	}
	return true
	//_, ok := ignoreCharsSet[char]
	//return ok
}

// n-gram 分词
func nGramSplit(str string, n int, consumer func(token string, pos int) error) {
	left := 0
	chars := []rune(str)
	for i, ch := range chars {
		if isIgnoredChar(ch) {
			left = i + 1
			continue
		} else if i-left+1 == n {
			err := consumer(string(chars[left:i+1]), left)
			if err != nil {
				log.Print(err.Error())
			}
			left++
		}
	}
}
