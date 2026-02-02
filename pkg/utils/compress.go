package utils

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"errors"
	"github.com/bodgit/sevenzip"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

func UnTarGzip(tarFile, dir string) error {
	srcFile, err := os.Open(tarFile)
	if err != nil {
		return err
	}
	defer srcFile.Close()
	gr, err := gzip.NewReader(srcFile)
	if err != nil {
		return err
	}
	defer gr.Close()
	tr := tar.NewReader(gr)
	for {
		hdr, err := tr.Next()
		if err != nil {
			if err == io.EOF {
				break
			} else {
				return err
			}
		}
		filename := filepath.Join(dir, hdr.Name)
		if hdr.FileInfo().IsDir() {
			if err = os.MkdirAll(filename, 777); err != nil {
				return err
			}
		} else {
			file, err := createFile(filename)
			if err != nil {
				return err
			}
			if _, err = io.Copy(file, tr); err != nil {
				file.Close()
				return err
			}
			file.Close()
		}
	}
	return nil
}
func createFile(name string) (*os.File, error) {
	err := os.MkdirAll(string([]rune(name)[0:strings.LastIndex(name, "/")]), 0777)
	if err != nil {
		return nil, err
	}
	return os.Create(name)
}

type Zip struct {
}

func (s *Zip) Zip(src string, dst string) error {
	zipfile, err := createFile(dst)
	if err != nil {
		return err
	}
	defer zipfile.Close()
	zipWriter := zip.NewWriter(zipfile)
	defer zipWriter.Close()
	stat, err := os.Stat(src)
	if err != nil {
		return err
	}
	if !stat.IsDir() {

		return nil
	}
	return nil
}
func (s *Zip) Unzip(src, dir string) error {
	r, err := zip.OpenReader(src)
	if err != nil {
		return err
	}
	defer r.Close()

	for _, f := range r.File {
		if f.FileInfo().IsDir() {
			if err = os.MkdirAll(filepath.Join(dir, f.Name), f.Mode()); err != nil {
				return err
			}
			continue
		}
		dst := filepath.Join(dir, f.Name)
		if err = s.extractFile(dst, f.Mode(), f); err != nil {
			return err
		}
	}
	return nil
}
func (s *Zip) UnzipEx(src, dir string, includes ...string) error {
	r, err := zip.OpenReader(src)
	if err != nil {
		return err
	}
	defer r.Close()

	for _, f := range r.File {
		if f.FileInfo().IsDir() {
			if err = os.MkdirAll(filepath.Join(dir, f.Name), f.Mode()); err != nil {
				return err
			}
			continue
		}
		if !s.includes(f.Name, includes...) {
			continue
		}
		dst := filepath.Join(dir, f.Name)
		if err = s.extractFile(dst, f.Mode(), f); err != nil {
			return err
		}
	}
	return nil
}
func (s *Zip) includes(name string, includes ...string) bool {
	for _, include := range includes {
		if name == include {
			return true
		}
	}
	return false
}
func (s *Zip) extractFile(dst string, mode fs.FileMode, f *zip.File) error {
	rc, err := f.Open()
	if err != nil {
		return err
	}
	defer rc.Close()
	file, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, mode)
	if err != nil {
		return err
	}
	defer file.Close()
	_, err = io.Copy(file, rc)
	return err
}
func (s *Zip) AddComment(srcFile, dstFile, comment string) error {
	zipFile, err := zip.OpenReader(srcFile)
	if err != nil {
		return err
	}
	defer zipFile.Close()
	tempFile, err := os.Create(dstFile)
	defer tempFile.Close()
	zipWriter := zip.NewWriter(tempFile)
	defer zipWriter.Close()
	// 复制原 ZIP 文件的内容到新的 ZIP 文件
	for _, file := range zipFile.File {
		if err = s.copy(file, zipWriter); err != nil {
			return err
		}
	}
	return zipWriter.SetComment(comment)
}
func (s *Zip) Hash(src string) ([]byte, error) {
	r, err := zip.OpenReader(src)
	if err != nil {
		return nil, err
	}
	defer r.Close()
	hash := sha256.New()
	for _, f := range r.File {
		if f.FileInfo().IsDir() {
			continue
		}
		open, err := f.Open()
		if err != nil {
			return nil, err
		}
		if _, err = io.Copy(hash, open); err != nil {
			return nil, err
		}
	}
	return hash.Sum(nil), nil
}
func (s *Zip) Comment(src string) (string, error) {
	r, err := zip.OpenReader(src)
	if err != nil {
		return "", err
	}
	defer r.Close()
	return r.Comment, nil
}
func (s *Zip) copy(file *zip.File, zipWriter *zip.Writer) error {
	newFileHeader := file.FileHeader
	fw, err := zipWriter.CreateHeader(&newFileHeader)
	if err != nil {
		return err
	}

	rc, err := file.Open()
	if err != nil {
		return err
	}
	defer rc.Close()
	// 创建新的文件头
	_, err = io.Copy(fw, rc)
	return err
}
func (s *Zip) ReadFile(src string, fileName string) ([]byte, error) {
	zipFile, err := zip.OpenReader(src)
	if err != nil {
		return nil, err
	}
	defer zipFile.Close()
	f, err := zipFile.Open(fileName)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return io.ReadAll(f)
}

// 加密函数
func (s *Zip) encrypt(data []byte, key []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	ciphertext := make([]byte, aes.BlockSize+len(data))
	iv := ciphertext[:aes.BlockSize]
	if _, err := io.ReadFull(rand.Reader, iv); err != nil {
		return nil, err
	}
	stream := cipher.NewCFBEncrypter(block, iv)
	stream.XORKeyStream(ciphertext[aes.BlockSize:], data)
	return ciphertext, nil
}

// 解密函数
func (s *Zip) decrypt(data []byte, key []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	if len(data) < aes.BlockSize {
		return nil, errors.New("ciphertext too short")
	}
	iv := data[:aes.BlockSize]
	data = data[aes.BlockSize:]
	stream := cipher.NewCFBDecrypter(block, iv)
	stream.XORKeyStream(data, data)
	return data, nil
}

// SevenZip 7zip解压缩
type SevenZip struct {
	Archive, Password string
}

func (s *SevenZip) Read(file string) ([]byte, error) {
	var r *sevenzip.ReadCloser
	var err error
	if s.Password == "" {
		r, err = sevenzip.OpenReader(s.Archive)
	} else {
		r, err = sevenzip.OpenReaderWithPassword(s.Archive, s.Password)
	}

	if err != nil {
		return nil, err
	}
	defer r.Close()
	for _, f := range r.File {
		if f.Name == file {
			if rc, err := f.Open(); err != nil {
				return nil, err
			} else {
				return io.ReadAll(rc)
			}
		}
	}
	return nil, errors.New("not found")
}
func (s *SevenZip) Extract(dir string) error {
	var r *sevenzip.ReadCloser
	var err error
	if s.Password == "" {
		r, err = sevenzip.OpenReader(s.Archive)
	} else {
		r, err = sevenzip.OpenReaderWithPassword(s.Archive, s.Password)
	}
	if err != nil {
		return err
	}
	defer r.Close()
	for _, f := range r.File {
		if f.FileInfo().IsDir() {
			if err = os.MkdirAll(filepath.Join(dir, f.Name), f.Mode()); err != nil {
				return err
			}
			continue
		}
		dst := filepath.Join(dir, f.Name)
		if err = s.extractFile(dst, f.Mode(), f); err != nil {
			return err
		}
	}
	return nil
}
func (s *SevenZip) extractFile(dst string, mode fs.FileMode, f *sevenzip.File) error {
	rc, err := f.Open()
	if err != nil {
		return err
	}
	defer rc.Close()
	file, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY, mode)
	if err != nil {
		return err
	}
	defer file.Close()
	_, err = io.Copy(file, rc)
	return err
}

// GZip 压缩数据
func GZip(data []byte) ([]byte, error) {
	var b bytes.Buffer
	gz := gzip.NewWriter(&b)
	if _, err := gz.Write(data); err != nil {
		return nil, err
	}
	if err := gz.Close(); err != nil {
		return nil, err
	}
	return b.Bytes(), nil
}

// UnGZip 解压缩数据
func UnGZip(data []byte) ([]byte, error) {
	r, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	defer r.Close()
	return io.ReadAll(r)
}
