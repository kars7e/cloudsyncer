package db

import (
	"time"

	"github.com/coopernurse/gorp"
)

// Keeps information about single file
type File struct {
	Id                int64  `db:"id"`
	Path              string `db:"path"`
	Parent            string `db:"parent"`
	IsDir             bool   `db:"is_dir"`
	CurrentRevisionId int64  `db:"current_revision_id"`
	IsRemoved         bool   `db:"is_removed"`
	UserId            int64  `db:"user_id"`
}

// Struct used for metadata sharing between client and server. It's NOT stored in database.
type Metadata struct {
	Size      int64     `json:"size"`
	Rev       int64     `json:"rev"`
	Name      string    `json:"Name"`
	IsDir     bool      `json:"is_dir"`
	Modified  time.Time `json:"modified"`
	IsRemoved bool      `json:"is_removed"`
	Path      string    `json:"path"`
	Hash      string    `json:"hash"`
}

// Returns the current revision of this file. If successful, returns pointer to Revision struct.
// Returns nil and error if error occured.
func (file *File) GetCurrentRevision() (revision *Revision, err error) {
	revision = new(Revision)
	if err := dbAccess.SelectOne(revision, "select * from revisions where id=?", file.CurrentRevisionId); err != nil {
		logger.Error(err)
		return nil, err
	}
	return revision, nil
}

// Returns all revisions associated with this file. If successful, returns pointer to Revision struct.
// Returns nil and error if error occured.
func (file *File) GetRevisions() (revisions []Revision, err error) {
	if _, err := dbAccess.Select(revisions, "select * from revisions where file_id=?", file.Id); err != nil {
		logger.Error(err)
		return nil, err
	}
	return revisions, nil
}

// Returns revision for this file which has the same size and hash.
// We assume that revisions of the same file with the same size and hash are identical (in terms of their contents).
// If successful, returns pointer to Revision struct.
// Returns nil and error if error has occured.
func (file *File) GetRevisionBySizeAndHash(size int64, hash string) (revision *Revision, err error) {
	count, err := dbAccess.SelectInt("select count(*) from revisions where file_id=? and hash = ? and size = ? ", file.Id, hash, size)
	if err != nil {
		logger.Error(err)
		return nil, err
	}
	if count < 1 {
		return nil, nil
	}
	revision = new(Revision)
	if err := dbAccess.SelectOne(revision, "select * from revisions where file_id=? and hash = ? and size = ? ", file.Id, hash, size); err != nil {
		logger.Error(err)
		return nil, err
	}
	return revision, nil

}

// Returns metadata for this revision.
// If successful, returns pointer to metadata struct.
// Returns nil and error if error has occured.
func (file *File) GetMetadata(rev *Revision) (metadata *Metadata, err error) {
	if rev == nil {
		rev, err = file.GetCurrentRevision()
		if err != nil {
			return nil, err
		}
	}
	metadata = new(Metadata)
	if err := dbAccess.SelectOne(metadata, `select revisions.hash Hash, files.path Path, revisions.name Name, files.is_dir IsDir, revisions.size Size, revisions.id Rev, revisions.modified Modified, files.is_removed IsRemoved
	                                     from files join revisions on files.id = revisions.file_id
																			 where revisions.id = ?`, rev.Id); err != nil {
		logger.Error(err)
		return nil, err
	}
	return metadata, nil
}

// Removes this file. Optional transaction might be given - files is then deleted in context of transaction.
// Returns nil if successful.
// Returnes error if error has occured.
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
	return nil
}

// Returns all children of this file. Returned value is a slice of File struct.
// If there are no children double nil is returned.
// Returns nil and error if error has occured.
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

// Returns revision for this value with given revision number.
// If revision does not exists, returns nil and error.
func (file *File) GetRevision(rev int64) (revision *Revision, err error) {
	revision = new(Revision)
	if err := dbAccess.SelectOne(revision, "select * from revisions where id=? and user_id = ? and file_id=?", rev, file.UserId, file.Id); err != nil {
		logger.Error(err)
		return nil, err
	}
	return revision, nil
}
