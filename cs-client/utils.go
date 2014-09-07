package main

import (
	"cloudsyncer/cs-client/db"
	"os"
	"os/user"
	"strings"
)

func getHomeDir() string {
	usr, _ := user.Current()
	return usr.HomeDir
}

func getConfigFileDir() string {
	return getHomeDir() + string(os.PathSeparator) + ".cloudsyncer"
}

func getDbFilePath() string {
	return getConfigFileDir() + string(os.PathSeparator) + "cloudsyncer_config.pb"
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

func getMetaForLocalFile(path string) (db.Metadata, error) {
	info, err := getOsInfo(path)
	var metadata db.Metadata
	if err != nil {
		return db.Metadata{}, err
	}
	metadata.Path = strings.Replace(path, appConfig["work_dir"], "", 1)
	metadata.IsDir = info.IsDir()
	metadata.IsRemoved = false
	metadata.Rev = 0
	metadata.Size = info.Size()
	metadata.Modified = info.ModTime()
	metadata.Name = info.Name()
	return metadata, nil
}
