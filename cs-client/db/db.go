// This package is responsible for storing and retreiving local state of files
package db

import (
	"database/sql"
	"errors"
	_ "fmt"

	"github.com/Sirupsen/logrus"
	"github.com/coopernurse/gorp"
	_ "github.com/mattn/go-sqlite3"
)

var dbAccess *gorp.DbMap
var logger *logrus.Logger

type gorpLogger struct {
	logger *logrus.Logger
}

func (log *gorpLogger) Printf(format string, v ...interface{}) {
	logger.Debugf(format, v...)
}

var (
	ErrEntityNotExists     = errors.New("Entity does not exists in database")
	ErrEntityAlreadyExists = errors.New("Entity already exists")
	ErrExist               = errors.New("file already exists")
	ErrNotExist            = errors.New("file does not exist")
)

// Initalization function. Sets database access and logger, creates tables if missing.
// Returns error if error has occured.
func InitDb(dbpath string, _logger *logrus.Logger) (err error) {
	if dbAccess != nil {
		return nil
	}
	logger = _logger
	db, err := sql.Open("sqlite3", dbpath)
	if err != nil {
		logger.Error(err)
		return err
	}

	dbAccess = &gorp.DbMap{Db: db, Dialect: gorp.SqliteDialect{}}
	//dbAccess.TraceOn("[gorp]", &gorpLogger{logger: logger})
	dbAccess.AddTableWithName(File{}, "files").SetKeys(true, "Id")
	dbAccess.AddTableWithName(Config{}, "config").SetKeys(true, "Id")
	if err = dbAccess.CreateTablesIfNotExists(); err != nil {
		logger.Fatal("Unable to create database Tables: " + err.Error())
		return err
	}
	return nil
}

// Closes connection to database.
func Close() {
	dbAccess.Db.Close()
}

// Resets database state by removing all records in files table. Use with caution.
// Returns error if error has occured.
func Reset() error {
	_, err := dbAccess.Exec("delete from files")
	return err
}

// Returns slice of File structs with synced attribute set to false, which means those files were not downloaded successfully.
// Returns double nil if no such files were found.
// Returns nil and error if error has occured.
func GetUnsyncedFiles() ([]File, error) {

	count, err := dbAccess.SelectInt("select count(*) from files where synced = ?", false)
	if err != nil {
		logger.Error(err)
		return nil, err
	}
	if count < 1 {
		return nil, nil
	}
	files := make([]File, 0)
	_, err = dbAccess.Select(&files, "select * from files where synced = ?", false)
	if err != nil {
		logger.Error(err)
		return nil, err
	}
	return files, nil

}

// Returns slice of File structs of not uploaded files, that means files without current_revision.
func GetNotUploadedFiles() ([]File, error) {

	count, err := dbAccess.SelectInt("select count(*) from files where current_revision = ?", 0)
	if err != nil {
		logger.Error(err)
		return nil, err
	}
	if count < 1 {
		return nil, nil
	}
	files := make([]File, 0)
	_, err = dbAccess.Select(&files, "select * from files where current_revision = ?", 0)
	if err != nil {
		logger.Error(err)
		return nil, err
	}
	return files, nil

}

// Returns pointer to file with given path, if such file exists.
func GetFileByPath(path string) (file *File, err error) {
	file = new(File)
	count, err := dbAccess.SelectInt("select count(*) from files where path = ?", path)
	if err != nil {
		logger.Error(err)
		return nil, err
	}
	if count < 1 {
		return nil, nil
	}
	if err := dbAccess.SelectOne(file, "select * from files where path = ?", path); err != nil {
		logger.Error(err)
		return nil, err
	}
	return file, nil
}
