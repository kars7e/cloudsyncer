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
	dbAccess.TraceOn("[gorp]", &gorpLogger{logger: logger})
	dbAccess.AddTableWithName(File{}, "files").SetKeys(true, "Id")
	dbAccess.AddTableWithName(Config{}, "config").SetKeys(true, "Id")
	if err = dbAccess.CreateTablesIfNotExists(); err != nil {
		logger.Fatal("Unable to create database Tables: " + err.Error())
		return err
	}
	return nil
}

func Close() {
	dbAccess.Db.Close()
}

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
