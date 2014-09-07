package main

import "cloudsyncer/cs-client/db"
import "code.google.com/p/go-uuid/uuid"

type FileOperation struct {
	Path       string
	Direction  OpDirection
	Type       OpType
	Attributes db.Metadata
	Id         string
}

func NewFileOperation() (fileOp FileOperation) {
	fileOp.Id = uuid.New()
	return
}

type OpDirection int
type OpType int

const (
	Create OpType = iota + 1
	Delete
	Move
	Modify
	ChangeAttrib
)
const (
	Incoming OpDirection = iota + 1
	Outgoing
)
