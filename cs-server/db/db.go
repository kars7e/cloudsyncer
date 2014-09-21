// This package is responsible for retreiving and storing files metadata.
package db

import (
	"database/sql"
	"errors"
	_ "fmt"

	"github.com/Sirupsen/logrus"
	"github.com/coopernurse/gorp"
	_ "github.com/go-sql-driver/mysql"
)

var dbAccess *gorp.DbMap
var logger *logrus.Logger

type gorpLogger struct {
	logger *logrus.Logger
}

func (log *gorpLogger) Printf(format string, v ...interface{}) {
	logger.Debugf(format, v...)
}

// Custom errors
var (
	ErrEntityNotExists     = errors.New("Entity does not exists in database")
	ErrEntityAlreadyExists = errors.New("Entity already exists")
	ErrExist               = errors.New("file already exists")
	ErrNotExist            = errors.New("file does not exist")
)

// Initalization function for package. Sets database access, creates missing tables and initalizes logger.
func InitDb(dbpath string, _logger *logrus.Logger) {
	logger = _logger
	db, err := sql.Open("mysql", "root:@/cloudsyncer?parseTime=true")
	if err != nil {
		logger.Fatal("Unable to open database: " + err.Error())
	}

	dbAccess = &gorp.DbMap{Db: db, Dialect: gorp.MySQLDialect{"InnoDB", "UTF8"}}
	//	dbAccess.TraceOn("[gorp]", &gorpLogger{logger: logger})
	dbAccess.AddTableWithName(User{}, "users").SetKeys(true, "Id")
	dbAccess.AddTableWithName(Session{}, "sessions").SetKeys(true, "Id")
	dbAccess.AddTableWithName(File{}, "files").SetKeys(true, "Id")
	dbAccess.AddTableWithName(Revision{}, "revisions").SetKeys(true, "Id")
	if err = dbAccess.CreateTablesIfNotExists(); err != nil {
		logger.Fatal("Unable to create database Tables: " + err.Error())
	}

}

// Closes database connection.
func Close() {
	dbAccess.Db.Close()
}
