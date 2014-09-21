package db

import (
	"time"

	"github.com/coopernurse/gorp"
)

// Revision struct keeps information about single revision of file.
type Revision struct {
	Id       int64     `db:"id"`
	Uuid     string    `db:"uuid"`
	Hash     string    `db:"hash"`
	Size     int64     `db:"size"`
	Created  int64     `db:"created"`
	Updated  int64     `db:"updated"`
	Modified time.Time `db:"modified"`
	FileId   int64     `db:"file_id"`
	IsDir    bool      `db:"is_dir"`
	Name     string    `db:"name"`
	UserId   int64     `db:"user_id"`
}

// Method invoked by gorp each time new Revision record is inserted into the database.
// Saves current time to Created and Updated attributes.
func (r *Revision) PreInsert(s gorp.SqlExecutor) error {
	r.Created = time.Now().UnixNano()
	r.Updated = r.Created
	return nil
}

// Method invoked by gorp each time existing Revision record is updated in the database.
// Saves current time to Updated attribute.
func (r *Revision) PreUpdate(s gorp.SqlExecutor) error {
	r.Updated = time.Now().UnixNano()
	return nil
}

// Returns pointer to Metadata struct on this revision. Returns nil and error if error has occured.
func (revision *Revision) GetMetadata() (metadata *Metadata, err error) {
	metadata = new(Metadata)
	if err := dbAccess.SelectOne(metadata, `select revisions.hash Hash, revisions.name Name, files.path Path, files.is_dir IsDir, revisions.size Size, revisions.id Rev, revisions.modified Modified, files.is_removed IsRemoved
	                                     from files join revisions on files.id = revisions.file_id
																			 where revisions.id = ?`, revision.Id); err != nil {
		logger.Error(err)
		return nil, err
	}
	return metadata, nil
}
