// This package is responsible for storing and retrieving file contents.
package storage

import (
	"bufio"
	"cloudsyncer/cs-server/config"
	"cloudsyncer/toolkit"
	"crypto/sha1"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
)

// interface which is returned by Retrieve function. Consists of 3 interfaces defined in io package.
type ReaderSeekerCloser interface {
	io.Reader
	io.Seeker
	io.Closer
}

// Takes uuid and file source as arguments, returns information about bytes stored and error if any.
// Currently this method stores files on local hard drive. In future it could be implemented to store files on other data media.
func Store(uuid string, source io.ReadCloser) (bytesCopied int64, err error) {
	dir := strings.Split(uuid, "-")
	if len(dir) < 2 || dir[0] == "" {
		return 0, errors.New("Error: invalid uuid")
	}
	if !toolkit.Exists(config.DATA_DIR) {
		os.MkdirAll(config.DATA_DIR, 0777)
	}
	dstDir := config.DATA_DIR + string(os.PathSeparator) + dir[0]
	if !toolkit.IsDirectory(dstDir) {
		if toolkit.Exists(dstDir) {
			return 0, errors.New("Error: destination path " + dstDir + " exists and is not a directory")
		}
		os.Mkdir(dstDir, 0777)
	}
	dst, err := os.Create(dstDir + string(os.PathSeparator) + uuid)
	if err != nil {
		return 0, errors.New("Error: unable to create destination file: " + err.Error())
	}
	defer dst.Close()
	bytesCopied, err = io.Copy(dst, source)
	if err != nil {
		return 0, errors.New("Error: unable to Copy to destination file: " + err.Error())
	}
	return bytesCopied, nil
}

// Returns SHA-1 hash for file identified by uuid string. Returns empty string if file does not exists.
func GetHash(uuid string) string {
	file, err := Retrieve(uuid)
	if err != nil {
		return ""
	}
	defer file.Close()
	reader := bufio.NewReader(file)
	sha1 := sha1.New()
	_, err = io.Copy(sha1, reader)
	if err != nil {
		return ""
	}
	return fmt.Sprintf("%x", sha1.Sum(nil))
}

// Takes uuid and returns file interface to read data. Returns nil and error if error has occured.
// Special interface has been declared for this, as currently in io package there is no interface with Read(), Close() and Seek() methods at once.
func Retrieve(uuid string) (file ReaderSeekerCloser, err error) {
	dir := strings.Split(uuid, "-")
	if len(dir) < 2 || dir[0] == "" {
		return nil, errors.New("Error: invalid uuid")
	}
	dstDir := config.DATA_DIR + string(os.PathSeparator) + dir[0]
	if !toolkit.IsDirectory(dstDir) {
		return nil, errors.New("Error: file does not exist")
	}
	file, err = os.Open(dstDir + string(os.PathSeparator) + uuid)
	if err != nil {
		return nil, errors.New("Error: unable to open the file: " + err.Error())
	}
	return file, nil

}
