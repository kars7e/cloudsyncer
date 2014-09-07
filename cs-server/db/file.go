package db

import (
	"time"

	"github.com/coopernurse/gorp"
)

type File struct {
	Id                int64  `db:"id"`
	Path              string `db:"path"`
	Parent            string `db:"parent"`
	IsDir             bool   `db:"is_dir"`
	CurrentRevisionId int64  `db:"current_revision_id"`
	IsRemoved         bool   `db:"is_removed"`
	UserId            int64  `db:"user_id"`
}

type Metadata struct {
	Size      int64     `json:"size"`
	Rev       int64     `json:"rev"`
	Name      string    `json:"Name"`
	IsDir     bool      `json:"is_dir"`
	Modified  time.Time `json:"modified"`
	IsRemoved bool      `json:"is_removed"`
	Path      string    `json:"path"`
}

func (file *File) GetCurrentRevision() (revision *Revision, err error) {
	revision = new(Revision)
	if err := dbAccess.SelectOne(revision, "select * from revisions where id=?", file.CurrentRevisionId); err != nil {
		logger.Error(err)
		return nil, err
	}
	return revision, nil
}

func (file *File) GetRevisions() (revisions []Revision, err error) {
	if _, err := dbAccess.Select(revisions, "select * from revisions where file_id=?", file.Id); err != nil {
		logger.Error(err)
		return nil, err
	}
	return revisions, nil
}

func (file *File) GetMetadata(rev *Revision) (metadata *Metadata, err error) {
	if rev == nil {
		rev, err = file.GetCurrentRevision()
		if err != nil {
			return nil, err
		}
	}
	metadata = new(Metadata)
	if err := dbAccess.SelectOne(metadata, `select files.path Path, revisions.name Name, files.is_dir IsDir, revisions.size Size, revisions.id Rev, revisions.modified Modified, files.is_removed IsRemoved
	                                     from files join revisions on files.id = revisions.file_id
																			 where revisions.id = ?`, rev.Id); err != nil {
		logger.Error(err)
		return nil, err
	}
	return metadata, nil
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
	file.IsRemoved = true
	_, err = tx.Update(file)
	if err != nil {
		tx.Rollback()
		return err
	}
	err = file.AddChange(tx)
	if err != nil {
		return err
	}
	return nil
}

func (file *File) AddChange(tx *gorp.Transaction) error {
	var newCursor int64 = 0
	count, err := tx.SelectInt("select count(*) from changes where user_id = ?", file.UserId)
	if err != nil {
		tx.Rollback()
		return err
	}
	if count > 0 {
		newCursor, err = tx.SelectInt("select max(cursor_new) from changes where user_id = ?", file.UserId)
		if err != nil {
			tx.Rollback()
			return err
		}
	}
	change := Change{FileId: file.Id, UserId: file.UserId, CursorOld: newCursor, CursorNew: newCursor + 1}
	err = tx.Insert(&change)
	if err != nil {
		tx.Rollback()
		return err
	}
	return nil
}
func (file *File) GetChildren() (children []File, err error) {
	count, err := dbAccess.SelectInt("select count(*) "+
		"from files "+
		"where files.user_id = ? and files.is_removed = 0 and files.parent = ? and files.id != ?", file.UserId, file.Path, file.Id)
	if err != nil {
		logger.Error(err)
		return nil, err
	}
	if count < 1 {
		return nil, nil
	}
	if _, err := dbAccess.Select(&children, `select *
	                                     from files
																			 where files.user_id = ? and files.is_removed = 0 and files.parent = ? and files.id !=  ?`, file.UserId, file.Path, file.Id); err != nil {
		logger.Error(err)
		return nil, err
	}
	return children, nil

}
