package main

import (
	"cloudsyncer/cs-client/db"
	"cloudsyncer/toolkit"
	"log"
	"os"
	"os/signal"
	"path"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"gopkg.in/fsnotify.v1"
)

type Watcher struct {
	operations      chan FileOperation
	excludedFolders []string
	path            string
	watcher         *fsnotify.Watcher
	worker          *Worker
}

func NewWatcher(path string, operations chan FileOperation, worker *Worker) *Watcher {
	w := Watcher{operations: operations, path: path}
	w.worker = worker
	w.excludedFolders = make([]string, 0)
	var err error
	w.watcher, err = fsnotify.NewWatcher()
	if err != nil {
		return nil
	}
	return &w
}

func (w *Watcher) AddExcludedFolder(path string) {
	w.excludedFolders = append(w.excludedFolders, path)
}

func (w *Watcher) Init() (err error) {
	log.Printf("Adding main folder %s to watcher", w.path)
	if err = w.watcher.Add(w.path); err != nil {
		return
	}
	w.registerExit()
	err = filepath.Walk(w.path, w.returnWalker())
	return
}

func (w *Watcher) Watch(wg *sync.WaitGroup) {
	wg.Add(1)
	go func() {
		defer wg.Done()
		defer w.watcher.Close()
		for {
			select {
			case ev, ok := <-w.watcher.Events:
				log.Println("event:", ev)
				if !ok {
					w.watcher.Events = nil
				}
				switch {
				case ev.Name == "":
					break
				case ev.Op&fsnotify.Create == fsnotify.Create:
					if toolkit.IsDirectory(ev.Name) {
						w.watcher.Add(ev.Name)
					}
					metadata, err := getMetaForLocalFile(ev.Name)
					if err != nil {
						log.Printf("Received error when reading metadata for file %s. Error: %s", ev.Name, err)
						metadata.IsRemoved = true
						metadata.Modified = time.Now()
						metadata.Name = path.Base(ev.Name)
					}
					op := NewFileOperation()
					op.Path = ev.Name
					op.Direction = Outgoing
					op.Type = Create
					op.Attributes = metadata
					w.operations <- op
				case ev.Op&fsnotify.Remove == fsnotify.Remove || ev.Op&fsnotify.Rename == fsnotify.Rename:
					var metadata db.Metadata
					metadata.IsRemoved = true
					metadata.Name = path.Base(ev.Name)
					metadata.Modified = time.Now()
					op := NewFileOperation()
					op.Path = ev.Name
					op.Direction = Outgoing
					op.Type = Delete
					op.Attributes = metadata
					w.operations <- op
				case ev.Op&fsnotify.Write == fsnotify.Write && ev.Op&fsnotify.Remove != fsnotify.Remove:
					metadata, err := getMetaForLocalFile(ev.Name)
					if err != nil {
						log.Printf("Received error when reading metadata for file %s. Error: %s", ev.Name, err)
						metadata.IsRemoved = true
						metadata.Modified = time.Now()
						metadata.Name = path.Base(ev.Name)
					}
					op := NewFileOperation()
					op.Path = ev.Name
					op.Direction = Outgoing
					op.Type = Modify
					op.Attributes = metadata
					w.operations <- op
				case ev.Op&fsnotify.Chmod == fsnotify.Chmod:

					metadata, err := getMetaForLocalFile(ev.Name)
					if err != nil {
						log.Printf("Received error when reading metadata for file %s. Error: %s", ev.Name, err)
						metadata.IsRemoved = true
						metadata.Modified = time.Now()
						metadata.Name = path.Base(ev.Name)
					}

					op := NewFileOperation()
					op.Path = ev.Name
					op.Direction = Outgoing
					op.Type = ChangeAttrib
					op.Attributes = metadata
					w.operations <- op
				}
			case err, ok := <-w.watcher.Errors:
				if err != nil {
					log.Println("error:", err)
				}
				if !ok {
					w.watcher.Errors = nil
				}

			}
			if w.watcher.Events == nil || w.watcher.Errors == nil {
				break
			}

		}
	}()
}

func (w *Watcher) registerExit() {
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)
	go func() {
		for _ = range c {
			log.Printf("Exiting...")
			w.watcher.Close()
		}
	}()
}

func (w *Watcher) returnWalker() filepath.WalkFunc {
	return func(path string, info os.FileInfo, err error) error {
		if err != nil {
			log.Printf("Error walking into path %s", path)
			return err
		}
		if info.IsDir() {
			log.Printf("Adding %s to watcher", path)
			w.watcher.Add(path)

		}
		if path != appConfig["work_dir"] {

			relativePath := strings.Replace(path, appConfig["work_dir"], "", 1)
			dbFile, err := db.GetFileByPath(toolkit.NormalizePath(relativePath))
			if err != nil {
				logger.Debug("Error getting file from database ", err)
				return err
			}
			metadata, err := getMetaForLocalFile(path)
			if err != nil {
				log.Printf("Error getting metadata for %s")
				return err
			}
			op := NewFileOperation()
			op.Direction = Outgoing
			op.Path = toolkit.NormalizePath(relativePath)
			op.Attributes = metadata
			if dbFile == nil {
				op.Type = Create
				w.worker.SetPendingOperation(toolkit.NormalizePath(relativePath), op)
				log.Printf("New file/folder %s. Adding to local state", path)

			} else if dbFile.ModificationTime.Unix() < info.ModTime().Unix() {
				op.Type = ChangeAttrib
				op.Attributes.Rev = dbFile.CurrentRevision
				w.worker.SetPendingOperation(toolkit.NormalizePath(relativePath), op)
			}
		}

		return nil
	}
}
