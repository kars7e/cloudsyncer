package main

import (
	"cloudsyncer/cs-server/config"
	"cloudsyncer/cs-server/db"
	"cloudsyncer/cs-server/protocols/http"
	"os"
)

func main() {
	db.InitDb(config.DB_PATH, logger)
	http.SetLogger(logger)
	var err = http.RunInterface("", 9999)
	if err != nil {
		os.Exit(1)
	} else {
		os.Exit(0)
	}

}
