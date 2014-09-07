package main

import (
	"cloudsyncer/toolkit"
	"log"
	"time"
)

type Worker struct {
	operations        chan FileOperation
	deltas            chan Delta
	path              string
	client            *Client
	pendingOperations map[string]FileOperation
}

func NewWorker(operations chan FileOperation, deltas chan Delta, client *Client, path string) *Worker {
	w := Worker{operations: operations, path: path, client: client, pendingOperations: make(map[string]FileOperation)}
	return &w
}

func (w *Worker) SetPendingOperation(path string, op FileOperation) {
	w.pendingOperations[path] = op
}

func (w *Worker) DeletePendingOperation(path string) {
	delete(w.pendingOperations, path)
}

func (w *Worker) GetPendingOperation(path string) (op FileOperation, exists bool) {
	op, exists = w.pendingOperations[path]
	return
}

func (w *Worker) Work() {
	go func() {
		for op := range w.operations {
			if op.Direction == Incoming {
				w.doIncoming(op)
			} else {
				w.doOutgoing(op)
			}
		}
	}()

}

func (w *Worker) Sync(cursor string) error {
	delta, err := w.client.GetDelta(cursor)
	if err != nil {
		log.Printf("failed to get delta for cursor '%s', trying again in 3 secs", cursor)
		time.Sleep(3 * time.Second)
		delta, err = w.client.GetDelta(cursor)
		if err != nil {
			log.Printf("failed to get delta for cursor '%s', giving up", cursor)
			return err
		}
	}
	logger.Debugf("Received delta: %v", delta)
	appConfig["pending_cursor"] = string(delta.Cursor)
	if delta.Entries != nil {
		for _, entry := range delta.Entries {
			for key, metadata := range entry {
				logger.Debugf("%s : %s", key, metadata == nil)
				op := NewFileOperation()
				op.Direction = Incoming
				op.Path = key
				op.Attributes.Path = key
				if metadata == nil {
					op.Attributes.IsRemoved = true
					op.Attributes.Modified = time.Now()
					op.Type = Delete
				} else {
					op.Attributes = *metadata
					op.Type = Create
				}
				w.operations <- op
			}
		}
	}
	return nil

}

func (w *Worker) doIncoming(op FileOperation) {
	log.Printf("doIncoming: %#v", op)
}

func (w *Worker) doOutgoing(op FileOperation) {

	log.Printf("doOutgoing: %#v", op)
	switch op.Type {
	case Create:
		if toolkit.IsDirectory(op.Path) {
			w.createDirectory(op.Path)
		} else {
			w.createFile(op.Path)
		}
	case Delete:
		w.remove(op.Path)
	}
}

func (w *Worker) createDirectory(path string) {
	err := w.client.Mkdir(path)
	if err != nil {
		log.Printf("error during creating directory '%s': '%s'", path, err)
	} else {
		log.Printf("'%s' directory created successfully", path)
	}
}

func (w *Worker) remove(path string) {
	err := w.client.Remove(path)
	if err != nil {
		log.Printf("error during removing path '%s': '%s'", path, err)
	} else {
		log.Printf("'%s' path removed successfully", path)
	}
}

func (w *Worker) createFile(path string) {
	err := w.client.Upload(path)
	if err != nil {
		log.Printf("error during file upload '%s': '%s'", path, err)
	} else {
		log.Printf("'%s' uploaded successfully", path)
	}
}
