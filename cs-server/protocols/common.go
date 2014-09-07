package protocols

import "github.com/Sirupsen/logrus"

type  Protocol interface {
	RunProtocol(address string, port int) error
	SetLogger(logger *logrus.Logger)
}
