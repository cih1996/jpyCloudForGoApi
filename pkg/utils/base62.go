package utils

import (
	"fmt"
	"math/big"
	"strings"
)

const base62Chars = "0123456789ABCDEFGHJKLMNOPQRSTUVWXYZabcdefghijkmnpqrstuvwxyz"

func Base62Encode(value int64) string {
	base := int64(len(base62Chars))
	result := ""
	for value > 0 {
		index := value % base
		result = string(base62Chars[index]) + result
		value /= base
	}
	return result
}
func Base62Decode(str string) (int64, error) {
	base := int64(len(base62Chars))
	result := int64(0)
	for _, char := range str {
		index := strings.IndexRune(base62Chars, char)
		if index == -1 {
			return 0, fmt.Errorf("invalid character in input: %c", char)
		}
		result = result*base + int64(index)
	}
	return result, nil
}
func Base62EncodeBytes(input []byte) string {
	value := new(big.Int).SetBytes(input)
	base := big.NewInt(int64(len(base62Chars)))
	zero := big.NewInt(0)
	result := ""
	mod := new(big.Int)
	for value.Cmp(zero) > 0 {
		value.DivMod(value, base, mod)
		result = string(base62Chars[mod.Int64()]) + result
	}
	return result
}
func Base62DecodeBytes(str string) ([]byte, error) {
	base := big.NewInt(int64(len(base62Chars)))
	result := big.NewInt(0)
	for _, char := range str {
		index := strings.IndexRune(base62Chars, char)
		if index == -1 {
			return nil, fmt.Errorf("invalid character in input: %c", char)
		}
		result.Mul(result, base)
		result.Add(result, big.NewInt(int64(index)))
	}
	return result.Bytes(), nil
}
