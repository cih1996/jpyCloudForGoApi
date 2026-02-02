package utils

import (
	"crypto"
	"crypto/aes"
	"crypto/cipher"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/sha512"
	"errors"
	"golang.org/x/crypto/poly1305"
	"hash"
	"io"
	"strconv"
)

// RsaEncrypt 使用 RSA 混合加密
func RsaEncrypt(public crypto.PublicKey, rand io.Reader, in, s1, s2 []byte) ([]byte, error) {
	publicKey, ok := public.(*rsa.PublicKey)
	if !ok {
		return nil, errors.New("private key not *rsa.PublicKey")
	}
	// 生成一个随机的对称密钥（AES-256）
	symmetricKey := make([]byte, 32) // 32 bytes = 256 bits
	if _, err := io.ReadFull(rand, symmetricKey); err != nil {
		return nil, err
	}

	// 使用 RSA 加密对称密钥
	encryptedKey, err := rsa.EncryptOAEP(sha256.New(), rand, publicKey, symmetricKey, s1)
	if err != nil {
		return nil, err
	}

	// 使用对称密钥加密数据
	ciphertext, err := encryptSymmetric(rand, in, symmetricKey)
	if err != nil {
		return nil, err
	}
	var key [32]byte
	copy(key[:], symmetricKey)
	// 计算消息认证码（MAC）
	mac := sumTag(ciphertext, s2, &key)

	// 输出格式：加密的对称密钥 + 密文 + MAC
	out := make([]byte, len(encryptedKey)+len(ciphertext)+len(mac))
	copy(out, encryptedKey)
	copy(out[len(encryptedKey):], ciphertext)
	copy(out[len(encryptedKey)+len(ciphertext):], mac[:])
	return out, nil
}

// RsaDecrypt 使用 RSA 混合解密
func RsaDecrypt(private crypto.PrivateKey, in, s1, s2 []byte) ([]byte, error) {
	privateKey, ok := private.(*rsa.PrivateKey)
	if !ok {
		return nil, errors.New("private key not *ecdsa.PrivateKey")
	}

	keySize := privateKey.Size()
	if len(in) < keySize+16 {
		return nil, errors.New("message too short")
	}
	encryptedKey := in[:keySize]
	ciphertext := in[keySize : len(in)-16]
	mac := in[len(in)-16:]

	// 使用 RSA 解密对称密钥
	symmetricKey, err := rsa.DecryptOAEP(sha256.New(), rand.Reader, privateKey, encryptedKey, s1)
	if err != nil {
		return nil, err
	}
	if len(symmetricKey) != 32 {
		return nil, errors.New("symmetric key length error")
	}
	match := verifyTag(to16ByteArray(mac), ciphertext, s2, to32ByteArray(symmetricKey))
	if !match {
		return nil, errors.New("message tags don't match")
	}

	// 使用对称密钥解密数据
	plaintext, err := decryptSymmetric(ciphertext, symmetricKey)
	if err != nil {
		return nil, err
	}

	return plaintext, nil
}

// EccEncrypt 加密
func EccEncrypt(public crypto.PublicKey, rand io.Reader, in, s1, s2 []byte) ([]byte, error) {
	pub, ok := public.(*ecdsa.PublicKey)
	if !ok {
		return nil, errors.New("private key not *ecdsa.PublicKey")
	}
	private, err := ecdsa.GenerateKey(pub.Curve, rand)
	if err != nil {
		return nil, err
	}
	curveName := pub.Curve.Params().Name
	var hashFunc hash.Hash
	if curveName == "P-521" {
		hashFunc = sha512.New()
	} else {
		hashFunc = sha256.New()
	}
	keySize := hashFunc.Size() / 2

	shared, err := deriveShared(private, pub, keySize)
	if err != nil {
		return nil, err
	}
	K := kdf(hashFunc, shared, s1)
	Ke := K[:keySize]
	Km := K[keySize:]
	if len(Km) < 32 {
		// Hash K_m so that it's 32 bytes long (required for Poly1305)
		hashFunc.Write(Km)
		Km = hashFunc.Sum(nil)
		hashFunc.Reset()
	}

	c, err := encryptSymmetric(rand, in, Ke)
	if err != nil {
		return nil, err
	}
	tag := sumTag(c, s2, to32ByteArray(Km))

	R := elliptic.Marshal(pub.Curve, private.PublicKey.X, private.PublicKey.Y)
	out := make([]byte, len(R)+len(c)+len(tag))
	copy(out, R)
	copy(out[len(R):], c)
	copy(out[len(R)+len(c):], tag[:])
	return out, nil
}

// EccDecrypt 解密
func EccDecrypt(private crypto.PrivateKey, in, s1, s2 []byte) ([]byte, error) {
	prv, ok := private.(*ecdsa.PrivateKey)
	if !ok {
		return nil, errors.New("private key not *ecdsa.PrivateKey")
	}
	curveName := prv.PublicKey.Curve.Params().Name
	var hashFunc hash.Hash
	if curveName == "P-521" {
		hashFunc = sha512.New()
	} else {
		hashFunc = sha256.New()
	}
	keySize := hashFunc.Size() / 2

	var messageStart int
	macLen := poly1305.TagSize

	if in[0] == 2 || in[0] == 3 || in[0] == 4 {
		messageStart = (prv.PublicKey.Curve.Params().BitSize + 7) / 4
		if len(in) < (messageStart + macLen + 1) {
			return nil, errors.New("invalid message")
		}
	} else {
		return nil, errors.New("invalid public key")
	}

	if curveName == "P-521" {
		// P-521 curve is serialized into 133 bytes, above formula yields size of only 132, therefore we must add 1
		// P-256 curve is serialized into 65 bytes, above formula yields correct result
		messageStart++
	}

	messageEnd := len(in) - macLen

	R := new(ecdsa.PublicKey)
	R.Curve = prv.PublicKey.Curve
	R.X, R.Y = elliptic.Unmarshal(R.Curve, in[:messageStart])
	if R.X == nil {
		return nil, errors.New("invalid public key. Maybe you didn't specify the right mode")
	}
	if !R.Curve.IsOnCurve(R.X, R.Y) {
		return nil, errors.New("invalid curve")
	}

	shared, err := deriveShared(prv, R, keySize)
	if err != nil {
		return nil, err
	}
	K := kdf(hashFunc, shared, s1)

	Ke := K[:keySize]
	Km := K[keySize:]
	if len(Km) < 32 {
		// Hash K_m so that it's 32 bytes long (required for Poly1305)
		hashFunc.Write(Km)
		Km = hashFunc.Sum(nil)
		hashFunc.Reset()
	}

	match := verifyTag(to16ByteArray(in[messageEnd:]), in[messageStart:messageEnd], s2, to32ByteArray(Km))
	if !match {
		return nil, errors.New("message tags don't match")
	}
	return decryptSymmetric(in[messageStart:messageEnd], Ke)
}

func deriveShared(private *ecdsa.PrivateKey, public *ecdsa.PublicKey, keySize int) ([]byte, error) {
	if private.PublicKey.Curve != public.Curve {
		return nil, errors.New("curves don't match")
	}
	if 2*keySize > (public.Curve.Params().BitSize+7)/8 {
		return nil, errors.New("shared key length is too long")
	}
	x, _ := public.Curve.ScalarMult(public.X, public.Y, private.D.Bytes())
	if x == nil {
		return nil, errors.New("scalar multiplication resulted in infinity")
	}
	shared := x.Bytes()
	return shared, nil
}
func kdf(hash hash.Hash, shared, s1 []byte) []byte {
	hash.Write(shared)
	if s1 != nil {
		hash.Write(s1)
	}
	key := hash.Sum(nil)
	hash.Reset()
	return key
}
func sumTag(in, shared []byte, key *[32]byte) [16]byte {
	var out [16]byte
	poly1305.Sum(&out, append(in, shared...), key)
	return out
}
func verifyTag(mac *[16]byte, in, shared []byte, key *[32]byte) bool {
	return poly1305.Verify(mac, append(in, shared...), key)
}
func getCryptoRandVec(rand io.Reader, len int) ([]byte, error) {
	out := make([]byte, len)
	_, err := io.ReadFull(rand, out)
	if err != nil {
		return nil, err
	}
	return out, nil
}
func to32ByteArray(in []byte) *[32]byte {
	if len(in) != 32 {
		panic("Input array size does not match. Expected 32, but got " + strconv.Itoa(len(in)))
	}
	var out [32]byte
	for i := 0; i < 32; i++ {
		out[i] = in[i]
	}
	return &out
}
func to16ByteArray(in []byte) *[16]byte {
	if len(in) != 16 {
		panic("Input array size does not match. Expected 16, but got " + strconv.Itoa(len(in)))
	}
	var out [16]byte
	for i := 0; i < 16; i++ {
		out[i] = in[i]
	}

	return &out
}
func encryptSymmetric(rand io.Reader, in, key []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	nonce, err := getCryptoRandVec(rand, aes.BlockSize)
	if err != nil {
		return nil, err
	}
	c := cipher.NewCTR(block, nonce)
	out := make([]byte, len(in))
	c.XORKeyStream(out, in)
	out = append(nonce, out...)
	return out, nil
}
func decryptSymmetric(in, key []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	c := cipher.NewCTR(block, in[:aes.BlockSize])
	out := make([]byte, len(in)-aes.BlockSize)
	c.XORKeyStream(out, in[aes.BlockSize:])
	return out, nil
}
