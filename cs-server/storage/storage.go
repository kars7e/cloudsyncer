package storage

import (
	"cloudsyncer/cs-server/config"
	"cloudsyncer/toolkit"
	"errors"
	"io"
	"os"
	"strings"
)

func Store(uuid string, source io.ReadCloser) (bytesCopied int64, err error) {
	dir := strings.Split(uuid, "-")
	if len(dir) < 2 || dir[0] == "" {
		return 0, errors.New("Error: invalid uuid")
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

func Retrieve(uuid string) (file io.ReadSeeker, err error) {
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
