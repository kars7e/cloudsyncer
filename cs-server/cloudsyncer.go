package main

import (
	"cloudsyncer/cs-server/config"
	"cloudsyncer/cs-server/db"
	"cloudsyncer/cs-server/server"
	"os"
)

func main() {
	db.InitDb(config.DB_PATH, logger)
	server.SetLogger(logger)
	var err = server.Serve("", 9999)
	if err != nil {
		os.Exit(1)
	} else {
		os.Exit(0)
	}

}
