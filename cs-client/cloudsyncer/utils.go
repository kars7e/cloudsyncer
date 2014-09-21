package cloudsyncer

import (
	"bufio"
	"cloudsyncer/cs-client/db"
	"crypto/sha1"
	"fmt"
	"io"
	"os"
	"os/user"
	"strings"
)

func getHomeDir() string {
	usr, _ := user.Current()
	return usr.HomeDir
}

func getConfigFileDir() string {
	if confPath == "" {
		return getHomeDir() + string(os.PathSeparator) + ".cloudsyncer"
	} else {
		return confPath
	}
}

func getDbFilePath() string {
	return getConfigFileDir() + string(os.PathSeparator) + "cloudsyncer.db"
}

func getRelativePath(path string) string {
	path = strings.Replace(path, appConfig["work_dir"], "", 1)
	return strings.ToLower(path)
}

func getOsInfo(path string) (os.FileInfo, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	fileinfo, err := file.Stat()
	if err != nil {
		return nil, err
	}
	return fileinfo, nil
}

func getTmpDir() string {
	return appConfig["work_dir"] + string(os.PathSeparator) + ".cloudsyncer_cache"
}

func getMetaForLocalFile(path string) (db.Metadata, error) {
	info, err := getOsInfo(path)
	var metadata db.Metadata
	if err != nil {
		return db.Metadata{}, err
	}
	metadata.Path = strings.Replace(strings.Replace(path, appConfig["work_dir"], "", 1), string(os.PathSeparator), "/", -1)
	metadata.IsDir = info.IsDir()
	metadata.IsRemoved = false
	metadata.Rev = 0
	metadata.Size = info.Size()
	metadata.Modified = info.ModTime()
	metadata.Name = info.Name()
	if !info.IsDir() {

		filePtr, err := os.Open(path)
		if err != nil {
			return db.Metadata{}, err
		}
		defer filePtr.Close()
		reader := bufio.NewReader(filePtr)
		sha1 := sha1.New()
		_, err = io.Copy(sha1, reader)
		if err != nil {
			return db.Metadata{}, err
		}

		metadata.Hash = fmt.Sprintf("%x", sha1.Sum(nil))
	}
	return metadata, nil
}
