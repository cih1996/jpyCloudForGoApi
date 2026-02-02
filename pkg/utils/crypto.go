package utils

import (
	"bytes"
	"crypto"
	"crypto/aes"
	"crypto/cipher"
	"crypto/ecdsa"
	"crypto/hmac"
	"crypto/md5"
	"crypto/rsa"
	"crypto/sha1"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/hex"
	"encoding/pem"
	"errors"
	"fmt"
	"hash/crc32"
	"io"
	"math/rand"
	"os"
	"reflect"
)

func SaveCertificate(cert *x509.Certificate, filename string, isPem bool) error {
	if cert == nil {
		return errors.New("证书无效,保存失败")
	}
	var buf []byte
	if isPem {
		pemBlock := &pem.Block{
			Type:  "CERTIFICATE",
			Bytes: cert.Raw,
		}
		buf = pem.EncodeToMemory(pemBlock)
	} else {
		buf = cert.Raw
	}
	return os.WriteFile(filename, buf, 0644)
}
func SavePrivateKey(privateKey crypto.PrivateKey, filename string, isPem bool) error {
	if privateKey == nil {
		return errors.New("秘钥无效,保存失败")
	}
	var buf, der []byte
	var err error
	switch key := privateKey.(type) {
	case *rsa.PrivateKey:
		der = x509.MarshalPKCS1PrivateKey(key)
	case *ecdsa.PrivateKey:
		der, err = x509.MarshalECPrivateKey(key)
		if err != nil {
			return err
		}
	default:
		return errors.New("unsupported private key type")
	}

	if isPem {
		var pemBlockType string
		switch privateKey.(type) {
		case *rsa.PrivateKey:
			pemBlockType = "RSA PRIVATE KEY"
		case *ecdsa.PrivateKey:
			pemBlockType = "EC PRIVATE KEY"
		}
		pemBlock := &pem.Block{
			Type:  pemBlockType,
			Bytes: der,
		}
		buf = pem.EncodeToMemory(pemBlock)
	} else {
		buf = der
	}
	return os.WriteFile(filename, buf, 0644)
}

// LoadCertificate 从文件中加载证书,自动识别pem和DER格式
func LoadCertificate(filename string) (*x509.Certificate, error) {
	buf, err := os.ReadFile(filename)
	if err != nil {
		return nil, err
	}
	return ParserCertificate(buf)
}

// ParserCertificate 解码der证书,自动识别pem和DER格式
func ParserCertificate(buf []byte) (*x509.Certificate, error) {
	var der []byte
	if p, _ := pem.Decode(buf); p != nil {
		if p.Type != "CERTIFICATE" {
			return nil, errors.New("pem数据类型错误")
		}
		der = p.Bytes
	} else {
		der = buf
	}
	return x509.ParseCertificate(der)
}

// LoadPrivateKey 从文件中加载私钥,自动识别pem和DER格式
func LoadPrivateKey(filename string) (crypto.PrivateKey, error) {
	buf, err := os.ReadFile(filename)
	if err != nil {
		return nil, err
	}
	return ParserPrivateKey(buf)
}

// ParserPrivateKey 解码der私钥,自动识别pem和DER格式
func ParserPrivateKey(buf []byte) (crypto.PrivateKey, error) {
	if block, _ := pem.Decode(buf); block != nil {
		switch block.Type {
		case "PRIVATE KEY":
			privateKey, err := x509.ParsePKCS8PrivateKey(block.Bytes)
			if err != nil {
				return nil, err
			}
			return privateKey, nil
		case "RSA PRIVATE KEY":
			privateKey, err := x509.ParsePKCS1PrivateKey(block.Bytes)
			if err != nil {
				return nil, err
			}
			return privateKey, nil
		case "EC PRIVATE KEY":
			privateKey, err := x509.ParseECPrivateKey(block.Bytes)
			if err != nil {
				return nil, err
			}
			return privateKey, nil
		default:
			return nil, errors.New("unsupported private key type in PEM file")
		}
	}

	key, err := x509.ParsePKCS1PrivateKey(buf)
	if err == nil {
		return key, nil
	}
	return x509.ParseECPrivateKey(buf)
}

func PKCS7Padding(ciphertext []byte, blockSize int) []byte {
	padding := blockSize - len(ciphertext)%blockSize
	padtext := bytes.Repeat([]byte{byte(padding)}, padding)
	return append(ciphertext, padtext...)
}

func PKCS7UnPadding(origData []byte) []byte {
	length := len(origData)
	unpadding := int(origData[length-1])
	return origData[:(length - unpadding)]
}

// AesEncrypt AES加密,CBC
func AesEncrypt(origData []byte, keyStr string) ([]byte, error) {
	key := []byte(keyStr)
	if len(key) > 16 {
		key = key[:16]
	} else {
		length := len(key)
		for i := 0; i < 16-length; i++ {
			key = append(key, byte(0x00))
		}
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	blockSize := block.BlockSize()
	origData = PKCS7Padding(origData, blockSize)
	blockMode := cipher.NewCBCEncrypter(block, key[:blockSize])
	crypted := make([]byte, len(origData))
	blockMode.CryptBlocks(crypted, origData)
	return crypted, nil
}

// AesDecrypt AES解密
func AesDecrypt(crypted []byte, keyStr string) ([]byte, error) {
	key := []byte(keyStr)
	if len(key) > 16 {
		key = key[:16]
	} else {
		length := len(key)
		for i := 0; i < 16-length; i++ {
			key = append(key, byte(0x00))
		}
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	blockSize := block.BlockSize()
	blockMode := cipher.NewCBCDecrypter(block, key[:blockSize])
	origData := make([]byte, len(crypted))
	blockMode.CryptBlocks(origData, crypted)
	origData = PKCS7UnPadding(origData)
	return origData, nil
}

func MakePwd(data string, salt string) string {
	str := salt + data + salt
	return Sha256(str)
}

func Sha256(data string) string {
	obj := sha256.Sum256([]byte(data))
	return hex.EncodeToString(obj[:])
}

func Sha1(data string) string {
	obj := sha1.New()
	obj.Write([]byte(data))
	return hex.EncodeToString(obj.Sum([]byte("")))
}
func FileSha256(filePath string) ([]byte, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("open file: %v", err)
	}
	defer file.Close()
	hash := sha256.New()
	if _, err = io.Copy(hash, file); err != nil {
		return nil, fmt.Errorf("sum MD5: %v", err)
	}
	return hash.Sum(nil), nil
}

func Sha256File(path string) string {
	f, err := os.Open(path)
	if err != nil {
		return ""
	}
	defer f.Close()
	obj := sha256.New()
	if _, err := io.Copy(obj, f); err != nil {
		return ""
	}
	return hex.EncodeToString(obj.Sum(nil))
}
func Hmac(key, data string) string {
	obj := hmac.New(md5.New, []byte(key))
	obj.Write([]byte(data))
	return hex.EncodeToString(obj.Sum([]byte("")))
}
func HmacBytes(key, data []byte) []byte {
	obj := hmac.New(md5.New, key)
	obj.Write(data)
	return obj.Sum(nil)
}
func HmacBase64(key, data string) string {
	obj := hmac.New(md5.New, []byte(key))
	obj.Write([]byte(data))
	return base64.StdEncoding.EncodeToString(obj.Sum([]byte("")))
}
func VerifyHmac(key, data, sig []byte) bool {
	expected := HmacBytes(key, data)
	return hmac.Equal(sig, expected)
}
func Md5(data string) string {
	obj := md5.New()
	obj.Write([]byte(data))
	return hex.EncodeToString(obj.Sum([]byte("")))
}

func Md5Byte(data []byte) string {
	obj := md5.New()
	obj.Write(data)
	return hex.EncodeToString(obj.Sum([]byte("")))
}
func Md5String(data string) []byte {
	obj := md5.New()
	obj.Write([]byte(data))
	return obj.Sum([]byte(""))
}
func Md5File(path string) string {
	f, err := os.Open(path)
	if err != nil {
		return ""
	}
	defer f.Close()
	obj := md5.New()
	if _, err := io.Copy(obj, f); err != nil {
		return ""
	}
	return hex.EncodeToString(obj.Sum(nil))
}
func CRC32Byte(data []byte) uint32 {
	obj := crc32.NewIEEE()
	obj.Write(data)
	return obj.Sum32()
}
func CRC32File(path string) uint32 {
	file, err := os.Open(path)
	if err != nil {
		return 0
	}
	defer file.Close()
	hash := crc32.NewIEEE()
	if _, err := io.Copy(hash, file); err != nil {
		return 0
	}
	return hash.Sum32()
}

// Xor 异或轻量加密/解密程序, key长度必须为4,padding为引入的随机数0~255之间
func Xor(data []byte, key []byte, padding int) []byte {
	generateKey := make([]byte, len(key))
	for i := 0; i < len(key); i++ {
		generateKey[i] = key[i%4] ^ byte((i*31)&padding)
	}

	length := len(data)
	encryptedData := make([]byte, length)
	for i := 0; i < length; i++ {
		encryptedData[i] = data[i] ^ generateKey[i] // 数据与密钥流进行异或
	}
	return encryptedData
}

// Xor2 异或轻量加密/解密程序
func Xor2(src, dst []byte, key []byte) error {
	if len(key) == 0 || len(src) == 0 {
		return errors.New("src and dst must have the same length")
	}
	if len(src) != len(dst) {
		return errors.New("src and dst must have the same length")
	}

	lk := len(key)
	for i := range src {
		k := key[i%lk] ^ byte(i*31) // 用 i 增加密钥流变化
		dst[i] = src[i] ^ k
	}
	return nil
}
func SignByPrivate(data []byte, key crypto.PrivateKey) ([]byte, error) {
	hashed := sha256.Sum256(data)
	return SignHashed(hashed[:], key)
}
func SignHashed(hashed []byte, key crypto.PrivateKey) ([]byte, error) {
	switch ct := key.(type) {
	case *rsa.PrivateKey:
		return rsa.SignPKCS1v15(rand.New(rand.NewSource(1)), ct, crypto.SHA256, hashed[:])
	case *ecdsa.PrivateKey:
		return ecdsa.SignASN1(rand.New(rand.NewSource(1)), ct, hashed)
	default:
		return nil, errors.New("unsupported key type")
	}
}
func VerifyByPublic(data, sign []byte, key crypto.PublicKey) error {
	hashed := sha256.Sum256(data)
	return VerifyHashed(hashed[:], sign, key)
}
func VerifyHashed(hashed []byte, sign []byte, key crypto.PublicKey) error {
	switch ct := key.(type) {
	case *rsa.PublicKey:
		if err := rsa.VerifyPKCS1v15(ct, crypto.SHA256, hashed[:], sign); err != nil {
			return err
		}
		return nil
	case *ecdsa.PublicKey:
		if ecdsa.VerifyASN1(ct, hashed[:], sign) {
			return nil
		}
		return errors.New("ecdsa verify failed")
	}
	return fmt.Errorf("unsupported certificate type %s", reflect.TypeOf(key).String())
}
