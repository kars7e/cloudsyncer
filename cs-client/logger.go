package main

import (
	"os"

	"github.com/Sirupsen/logrus"
)

var logger = logrus.New()

func init() {
	logger.Formatter = new(logrus.TextFormatter) // default
	logger.Out = os.Stdout
	logger.Level = logrus.Debug
}
