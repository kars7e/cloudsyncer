package cloudsyncer

import "cloudsyncer/cs-client/db"
import "code.google.com/p/go-uuid/uuid"

// Describes local file operation. Path is absolute path on local file system.
// Direction is always Outgoing. Type might be Delete, Create, Rename or Chmod.
// Attrbiutes holds Metadata of the file.
type FileOperation struct {
	Path       string
	Direction  OpDirection
	Type       OpType
	Attributes db.Metadata
	Id         string
}

// Creates and returns new FileOperation.
func NewFileOperation() (fileOp FileOperation) {
	fileOp.Id = uuid.New()
	return
}

// Direction of Operation. Currently always outgoing
type OpDirection int

// Types of Operation.
type OpType int

// Defines types of File Operation.
const (
	Create OpType = iota + 1
	Delete
	Move
	Modify
	ChangeAttrib
)

// Defines direction of File Operation.
const (
	Incoming OpDirection = iota + 1
	Outgoing
)
