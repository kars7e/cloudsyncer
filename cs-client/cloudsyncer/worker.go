package cloudsyncer

import (
	"cloudsyncer/cs-client/db"
	"cloudsyncer/toolkit"
	"errors"
	"io"
	"log"
	"os"
	"path"
	"strconv"
	"strings"

	"code.google.com/p/go-uuid/uuid"
)

// Worker is responsible for applying incoming changes.
// operations channel holds incoming local file operations.
// deltas channel hold incoming remote file operations.
// path holds current user work dir.
// client holds Client instance which is responsible for network operations.
// listener instance - kept here to restart listener after successful delta handling.
type Worker struct {
	operations        chan FileOperation
	deltas            chan Delta
	path              string
	client            *Client
	pendingOperations map[string]FileOperation
	listener          *Listener
}

// Creates and returns new worker instance with given parameters.
func NewWorker(operations chan FileOperation, deltas chan Delta, client *Client, listener *Listener, path string) *Worker {
	w := Worker{operations: operations, deltas: deltas, path: path, client: client, pendingOperations: make(map[string]FileOperation), listener: listener}
	return &w
}

// Sets pending operation
func (w *Worker) SetPendingOperation(path string, op FileOperation) {
	w.pendingOperations[path] = op
}

// removes pending operation
func (w *Worker) DeletePendingOperation(path string) {
	delete(w.pendingOperations, path)
}

// returns pending operation
func (w *Worker) GetPendingOperation(path string) (op FileOperation, exists bool) {
	op, exists = w.pendingOperations[path]
	return
}

// Initalizes database with initial state. Grabs current state from server, and creates record for each file, with synced set to false.
func (w *Worker) InitDb() error {
	if db.GetCfgValue("cursor") == "" {
		delta, err := w.client.GetDelta("")
		if err != nil {
			return err
		}
		for _, entry := range delta.Entries {
			for key, metadata := range entry {
				db.AddFile(key, metadata, false)
			}
		}
		db.SetCfgValue("cursor", delta.Cursor)
		log.Printf("InitDb cursor set to %s", delta.Cursor)
	}
	return nil
}

// Syncs state. Uploads not uploaded files, downloads not downloaded files.
func (w *Worker) Sync() error {
	log.Printf("Worker syncing...")
	notUploadedFiles, err := db.GetNotUploadedFiles()
	if err != nil {
		return err
	}
	for _, file := range notUploadedFiles {
		if file.IsDir {
			log.Printf("Found not uploaded file %s", file.Path)
			err = w.createRemoteFile((w.path + strings.Replace(file.Path, "/", string(os.PathSeparator), -1)), file.Metdata())
			if err != nil {
				return err
			}
		} else {

			log.Printf("Found not uploaded file %s", file.Path)
			err = w.createRemoteDirectory((w.path + strings.Replace(file.Path, "/", string(os.PathSeparator), -1)), file.Metdata())
			if err != nil {
				return err
			}
		}
	}
	unsyncedFiles, err := db.GetUnsyncedFiles()
	if err != nil {
		return err
	}
	if unsyncedFiles == nil {
		return nil
	}
	for _, file := range unsyncedFiles {
		log.Printf("Found unsynced file %s", file.Path)
		err = w.createLocalFile(file.Path)
		if err != nil {
			return err
		}

	}
	return nil
}

// Creates goroutine which handles incoming file operations
func (w *Worker) Work() {
	go func() {
		log.Print("worker waits for operations")
		for op := range w.operations {
			w.handleFileOp(op)
		}
	}()
	go func() {
		log.Print("worker waits for deltas")
		for delta := range w.deltas {
			log.Printf("received delta %v", delta)
			w.handleDelta(delta)
		}
		log.Print("skonczylem sie?")
	}()

}

func (w *Worker) handleDelta(delta Delta) {

	log.Printf("Worker received delta: %#v", delta)
	if delta.Reset {
		log.Print("would reset db")
		// db.Reset()
	}
	if delta.Entries != nil {
		log.Printf("Worker got %d entries in delta", len(delta.Entries))
		entryErr := make(map[string]error)
		for _, entry := range delta.Entries {
			for key, metadata := range entry {
				if !w.isNewEntry(key, metadata) {
					log.Printf("Delta entry already in state for %s", key)
					break
				}
				err := w.setMetadata(key, metadata, false)
				if err != nil {
					entryErr[key] = err
					break
				}
				if metadata == nil {
					err = w.RemoveFile(key)
					if err != nil {
						entryErr[key] = err
						break
					}
					continue
				}
				err = w.createLocalFile(key)
			}
		}
		log.Printf("len(entryerr): %d", len(entryErr))
		if len(entryErr) < 1 {
			log.Printf("worker handling delta No errors, setting cursor to %s", delta.Cursor)
			db.SetCfgValue("cursor", delta.Cursor)
			w.listener.Listen(delta.Cursor)
		}
		for key, err := range entryErr {
			log.Printf("There was an error handling delta for %s: %s", key, err)
		}
	}
}

func (w *Worker) setMetadata(key string, metadata *db.Metadata, synced bool) error {
	log.Printf("setMetadata(%s)", key)
	file, err := db.GetFileByPath(key)
	if err != nil {
		return err
	}
	if file == nil {
		file = new(db.File)
	}
	if file.CurrentRevision != 0 {
		file.ParentRevision = file.CurrentRevision
	}
	file.Path = metadata.Path
	file.Name = metadata.Name
	file.ModificationTime = metadata.Modified
	file.CurrentRevision = metadata.Rev
	file.IsDir = metadata.IsDir
	file.Parent = toolkit.Dir(file.Path)
	file.Size = metadata.Size
	file.Hash = metadata.Hash
	file.Synced = synced
	return file.Save()
}

func (w *Worker) createLocalFile(key string) (err error) {
	log.Printf("createLocalFile(%s)", key)
	file, err := db.GetFileByPath(key)
	if err != nil {
		return err
	}
	if file == nil {
		return errors.New("No file database when it was requested")
	}
	if file.IsDir == false {
		tmpFileName := getTmpDir() + string(os.PathSeparator) + uuid.New()
		out, err := os.Create(tmpFileName)
		if err != nil {
			log.Printf("Error creating tmp file %s: %s", tmpFileName, err)
			return err
		}
		defer out.Close()
		body, err := w.client.GetFile(key, strconv.FormatInt(file.CurrentRevision, 10))
		if err != nil {
			log.Printf("Error downloading file %s", err)
			return err
		}
		if body == nil {
			log.Printf("Error downloading file %s, empty body!!", key)
			return errors.New("Error downloading file %s, empty body!!")
		}
		defer body.Close()
		n, err := io.Copy(out, body)
		if err != nil {
			log.Printf("Error on copying file %s", err)
			return err
		}
		if n != file.Size {
			log.Printf("Size download mismatch n: %d, metadata: %d", n, file.Size)
			return errors.New("size mismatch")
		}
		log.Printf("createLocalFile parts. w.path: %s, toolkit.cleanpath: %s, path.Dir(key): %s, file.Name: %s", w.path, toolkit.OnlyCleanPath(strings.Replace(path.Dir(key), "/", string(os.PathSeparator), -1)), path.Dir(key), file.Name)
		targetpath := w.path + toolkit.OnlyCleanPath(strings.Replace(path.Dir(key), "/", string(os.PathSeparator), -1)) + string(os.PathSeparator) + file.Name
		log.Printf("Setting discard for %s", targetpath)
		discard[targetpath] = true // we need to say watcher to do not care about this remove operation

		err = file.Sync()
		if err != nil {
			log.Printf("Error on syncing in createLocalFile %s", err)
			return err
		}
		log.Printf("Renaming file from %s to %s", tmpFileName, targetpath)
		os.Rename(tmpFileName, targetpath)
		log.Print("Local file created succesfully: ", key)
		return nil
	} else {
		targetpath := w.path + toolkit.OnlyCleanPath(strings.Replace(path.Dir(key), "/", string(os.PathSeparator), -1)) + string(os.PathSeparator) + file.Name
		if err = os.MkdirAll(targetpath, 0777); err != nil {
			return err
		}
		file.Sync()
		log.Print("Folder created successfully: ", key)

	}

	return nil
}

// Updates metadata for given filepath
func (w *Worker) UpdateMetadata(key string, metadata *db.Metadata) error {
	return nil

}

// Removes file at given path
func (w *Worker) RemoveFile(relativePath string) error {
	return nil
}

func (w *Worker) handleFileOp(op FileOperation) {

	log.Printf("worker received File Operation: %#v", op)
	if w.isNewOperation(op) {
		log.Printf("got new operation: %#v", op)
		switch op.Type {
		case Create:
			if toolkit.IsDirectory(op.Path) {
				w.createRemoteDirectory(op.Path, op.Attributes)
			} else {
				w.createRemoteFile(op.Path, op.Attributes)
			}
		case Delete:
			w.removeRemoteFile(op.Path)
		}
	}
}

func (w *Worker) isNewOperation(op FileOperation) bool {
	file, err := db.GetFileByPath(op.Attributes.Path)
	if err != nil {
		log.Printf("Error retrieving file %s: %s", op.Attributes.Path, err)
		return true
	}
	if file == nil {
		log.Printf("No file record for  %s", op.Attributes.Path)
		if op.Type == Delete {
			return false
		} else {
			return true
		}
	}
	if op.Type == Delete {
		return true
	}
	if op.Attributes.Name != file.Name {
		log.Printf("File name mismatch  %s : %s", op.Attributes.Name, file.Name)
		return true
	}
	if op.Attributes.IsDir == file.IsDir && file.IsDir == true {
		log.Printf("Both directories, no new  %s : %s", op.Attributes.Name, file.Name)
		return false
	}

	if op.Attributes.IsDir != file.IsDir {
		log.Printf("File type (dir/file) mismatch  %s : %s", op.Attributes.Name, file.Name)
		return true
	}
	if op.Attributes.Size != file.Size {
		log.Printf("File size mismatch  %d : %d", op.Attributes.Size, file.Size)
		return true
	}

	if op.Attributes.Hash != file.Hash {
		log.Printf("File hash mismatch  %s : %s", op.Attributes.Hash, file.Hash)
		return true
	}
	return false

}

func (w *Worker) isNewEntry(key string, metadata *db.Metadata) bool {
	log.Printf("isNewEntry(%s)", key)
	file, err := db.GetFileByPath(key)
	if err != nil {
		log.Printf("isNewEntry: Error retrieving file %s: %s", key, err)
		return true
	}
	if file == nil {
		log.Printf("isNewEntry: No file record for  %s", key)
		if metadata == nil {
			return false
		} else {
			return true
		}
	}
	if metadata == nil {
		return true
	}
	if metadata.Name != file.Name {
		log.Printf("isNewEntry: File name mismatch  %s : %s", metadata.Name, file.Name)
		return true
	}
	if metadata.Size != file.Size {
		log.Printf("isNewEntry: File size mismatch  %d : %d", metadata.Size, file.Size)
		return true
	}

	if metadata.Hash != file.Hash {
		log.Printf("isNewEntry: File hash mismatch  %s : %s", metadata.Hash, file.Hash)
		return true
	}
	return false

}

func (w *Worker) createRemoteDirectory(path string, metadata db.Metadata) error {
	w.setMetadata(metadata.Path, &metadata, true)
	newMetadata, err := w.client.Mkdir(path)
	if err != nil {
		log.Printf("error during creating directory '%s': '%s'", path, err)
		return err
	}
	err = w.setMetadata(metadata.Path, &newMetadata, true)
	if err != nil {
		log.Printf("error during creating directory '%s': '%s'", path, err)
		return err
	}
	return nil
}

func (w *Worker) removeRemoteFile(path string) {
	err := w.client.Remove(path)
	if err != nil {
		log.Printf("error during removing path '%s': '%s'", path, err)
	} else {
		log.Printf("'%s' path removed successfully", path)
	}
}

func (w *Worker) createRemoteFile(path string, metadata db.Metadata) error {
	w.setMetadata(metadata.Path, &metadata, true)
	newMetadata, err := w.client.Upload(path)
	if err != nil {
		log.Printf("error during file upload '%s': '%s'", path, err)
		return err
	}
	w.setMetadata(metadata.Path, &newMetadata, true)
	return nil
}
