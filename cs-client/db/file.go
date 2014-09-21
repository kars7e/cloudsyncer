package db

import (
	"cloudsyncer/toolkit"
	"time"

	"github.com/coopernurse/gorp"
)

// Struct keeps file metadata for current state.
type File struct {
	Id               int64     `db:"id"`
	Path             string    `db:"path"`
	IsDir            bool      `db:"is_dir"`
	CurrentRevision  int64     `db:"current_revision"`
	ParentRevision   int64     `db:"parent_revision"`
	IsRemoved        bool      `db:"is_removed"`
	Parent           string    `db:"parent"`
	Name             string    `db:"name"`
	Size             int64     `db:"size"`
	Synced           bool      `db:"synced"`
	Hash             string    `db:"hash"`
	ModificationTime time.Time `db:"modification_time"`
	CreationTime     time.Time `db:"creation_time"`
	Created          int64     `db:"created"`
	Updated          int64     `db:"updated"`
}

// struct Metadata is used for exchanging files metadata with server. It is NOT stored in database.
type Metadata struct {
	Size      int64     `json:"size"`
	Rev       int64     `json:"rev"`
	Name      string    `json:"name"`
	IsDir     bool      `json:"is_dir"`
	Modified  time.Time `json:"modified"`
	IsRemoved bool      `json:"is_removed"`
	Path      string    `json:"path"`
	Hash      string    `json:"hash"`
}

// Adds file with given path and metadata to state.
func AddFile(path string, metadata *Metadata, synced bool) error {
	file := File{}
	file.Path = path
	file.IsDir = metadata.IsDir
	file.CurrentRevision = metadata.Rev
	file.Parent = toolkit.Dir(path)
	file.Name = metadata.Name
	file.Size = metadata.Size
	file.Synced = synced
	file.ModificationTime = metadata.Modified
	file.IsRemoved = false
	file.Hash = metadata.Hash
	return dbAccess.Insert(&file)
}

// Returns true if file at given path is synced
func IsSynced(path string) bool {
	var file File
	err := dbAccess.SelectOne(&file, "select * from files where path = ?", path)
	if err != nil {
		return true
	}
	return file.Synced

}

// Sets synced to true for this file
func (f *File) Sync() error {
	if !f.Synced {
		f.Synced = true

		_, err := dbAccess.Update(f)
		return err
	}
	return nil
}

// Returns Metadata struct for this file
func (f *File) Metdata() Metadata {
	var metadata Metadata
	metadata.Hash = f.Hash
	metadata.IsDir = f.IsDir
	metadata.Modified = f.ModificationTime
	metadata.Name = f.Name
	metadata.Path = f.Path
	metadata.Rev = f.CurrentRevision
	metadata.Size = f.Size
	return metadata
}

// Saves this file. If file is new, inserts new record. It updates record otherwise.
func (f *File) Save() error {
	if f.Id == 0 {
		return dbAccess.Insert(f)
	} else {
		_, err := dbAccess.Update(f)
		return err
	}
}

// Sets new modification time and updates record in database.
func (f *File) UpdateModificationTime(t time.Time) error {
	f.ModificationTime = t
	_, err := dbAccess.Update(&f)
	return err
}

// This function is called by Gorp each time new record is inserted into the database.
// Sets created and updated to current time.
func (f *File) PreInsert(s gorp.SqlExecutor) error {
	f.Created = time.Now().UnixNano()
	f.Updated = f.Created
	return nil
}

// This function is called by Gorp each time existing record is updated in the database.
// Sets updated to current time.
func (f *File) PreUpdate(s gorp.SqlExecutor) error {
	f.Updated = time.Now().UnixNano()
	return nil
}

// Removes file and all its children. Optional transaction might provided,
// In such case operation is invoked in context of transaction.
func (f *File) RemoveAll(tx *gorp.Transaction) (err error) {
	if f.IsDir {
		children, err := f.GetChildren()
		if err != nil {
			return err
		}
		for _, child := range children {
			err = (&child).RemoveAll(tx)
			if err != nil {
				return err
			}
		}
	}
	_, err = tx.Delete(&f)
	return

}

// Deletes current file.
func (f *File) Remove(tx *gorp.Transaction) (err error) {
	_, err = tx.Delete(&f)
	return
}

// Returns slice of File struct, files being children of current file.
func (file *File) GetChildren() (children []File, err error) {
	count, err := dbAccess.SelectInt(`select count(*) from files where files.parent = '?'`, file.Path)
	if err != nil {
		logger.Error(err)
		return nil, err
	}
	if count < 1 {
		return nil, nil
	}
	if _, err := dbAccess.Select(children, `select * from files where files.parent = ?`, file.Path); err != nil {
		logger.Error(err)
		return nil, err
	}
	return children, nil

}
