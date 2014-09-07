package toolkit

import (
	"crypto/rand"
	"crypto/sha1"
	"fmt"
	"os"
	"path"
	"strings"
)

func GetSha1(input []byte) string {
	var hasher = sha1.New()
	hasher.Write(input)
	return fmt.Sprintf("%x", hasher.Sum(nil))
}

func GetRandHex(length int) string {
	var buf = make([]byte, length)
	rand.Read(buf)
	var str = fmt.Sprintf("%x", buf)
	return str[:length-1]
}

func Exists(path string) bool {
	_, err := os.Stat(path)
	if err == nil {
		return true
	}
	return false
}

func IsDirectory(path string) bool {
	fi, err := os.Stat(path)
	if err == nil && fi.Mode().IsDir() {
		return true
	}
	return false
}

func CleanPath(filepath string) string {
	return strings.ToLower(path.Clean(filepath))
}

func NormalizePath(filepath string) string {
	return strings.ToLower(filepath)
}

func OnlyCleanPath(filepath string) string {
	return path.Clean(filepath)
}

func Dir(filepath string) string {
	filepath = path.Dir(filepath)
	if filepath == "." {
		filepath = "/"
	}
	return filepath
}
