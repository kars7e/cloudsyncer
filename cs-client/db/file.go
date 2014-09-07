package db

import (
	"time"

	"github.com/coopernurse/gorp"
)

type File struct {
	Id               int64     `db:"id"`
	Path             string    `db:"path"`
	IsDir            bool      `db:"is_dir"`
	CurrentRevision  int64     `db:"current_revision"`
	IsRemoved        bool      `db:"is_removed"`
	Parent           string    `db:"parent"`
	Name             string    `db:"name"`
	Size             int64     `db:"size"`
	Synced           bool      `db:"synced"`
	ModificationTime time.Time `db:"modification_time"`
	CreationTime     time.Time `db:"creation_time"`
	Created          int64     `db:"created"`
	Updated          int64     `db:"updated"`
}

type Metadata struct {
	Size      int64     `json:"size"`
	Rev       int64     `json:"rev"`
	Name      string    `json:"name"`
	IsDir     bool      `json:"is_dir"`
	Modified  time.Time `json:"modified"`
	IsRemoved bool      `json:"is_removed"`
	Path      string    `json:"path"`
}

func (f *File) PreInsert(s gorp.SqlExecutor) error {
	f.Created = time.Now().UnixNano()
	f.Updated = f.Created
	return nil
}

func (f *File) PreUpdate(s gorp.SqlExecutor) error {
	f.Updated = time.Now().UnixNano()
	return nil
}

func (file *File) Remove(tx *gorp.Transaction) (err error) {
	if file.IsDir {
		children, err := file.GetChildren()
		if err != nil {
			tx.Rollback()
			return err
		}
		for _, child := range children {
			err = (&child).Remove(tx)
			if err != nil {
				return err
			}
		}
	}
	return tx.Commit()
}

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
