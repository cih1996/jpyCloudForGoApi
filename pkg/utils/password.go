package utils

import (
	"crypto/rand"
	"math/big"
)

const (
	letters      = "0123456789abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"
	specialChars = "!@#$%^&*()-_=+[]{}<>?/|"
)

// GeneratePasswd 生成指定长度的随机字符串,length字符串长度,special特殊字符数量
func GeneratePasswd(length, special int) string {
	password := make([]byte, length)
	for i := 0; i < length-1; i++ {
		index, _ := rand.Int(rand.Reader, big.NewInt(int64(len(letters))))
		password[i] = letters[index.Int64()]
	}
	// 随机位置插入特殊符号
	for i := 0; i < special; i++ {
		index, _ := rand.Int(rand.Reader, big.NewInt(int64(length)))
		specialIndex := index.Int64()
		specialCharIndex, _ := rand.Int(rand.Reader, big.NewInt(int64(len(specialChars))))
		password[specialIndex] = specialChars[specialCharIndex.Int64()]
	}
	return string(password)
}
