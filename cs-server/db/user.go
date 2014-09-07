package db

import (
	"cloudsyncer/toolkit"
	"errors"
	"path"
	"time"
)

type User struct {
	Id       int64  `db:"id"`
	Username string `db:"username"`
	Salt     string `db:"salt"`
	Password string `db:"password"`
}

func (user *User) CheckPassword(password string) bool {
	if toolkit.GetSha1([]byte(user.Salt+password)) == user.Password {
		return true
	}
	return false
}

func GetUser(username string) *User {
	var user User
	if err := dbAccess.SelectOne(&user, "select * from users where username=?", username); err != nil {
		logger.Error(err)
		return nil
	}
	if user.Username == "" {
		return nil
	}
	return &user

}

func CreateUser(username string, password string) (*User, error) {
	if user := GetUser(username); user != nil {
		return nil, ErrEntityAlreadyExists
	}
	var salt = toolkit.GetRandHex(15)
	var user = User{Username: username, Password: toolkit.GetSha1([]byte(salt + password)), Salt: salt}
	var err = dbAccess.Insert(&user)
	if err != nil {
		logger.Error(err)
		return nil, err
	}
	return GetUser(username), nil
}
func (user *User) GetChildren(path string) (children []Metadata, err error) {
	count, err := dbAccess.SelectInt(`select count(*) 
	                                     from files join revisions on files.current_revision_id = revisions.id
																			 where files.user_id = ? and files.is_removed = 0 and files.parent = ? `, user.Id, path)
	if err != nil {
		logger.Error(err)
		return nil, err
	}
	if count < 1 {
		return nil, nil
	}
	if _, err := dbAccess.Select(&children, `select revisions.name Name, files.path Path, files.is_dir IsDir, revisions.size Size, revisions.id Rev, revisions.modified Modified
	                                     from files join revisions on files.current_revision_id = revisions.id
																			 where files.user_id = ? and files.is_removed = 0 and files.parent = ? `, user.Id, path); err != nil {
		logger.Error(err)
		return nil, err
	}
	return children, nil

}
func (user *User) GetCurrentState() ([]map[string]interface{}, error) {
	children, err := user.GetChildren("/")
	if err != nil {
		return nil, err
	}
	resp := make([]map[string]interface{}, 0)
	for _, child := range children {
		resp = append(resp, map[string]interface{}{child.Path: child})
	}
	return resp, nil
}

func (user *User) GetChangesFromCursor(cursor int64) ([]map[string]interface{}, int64, error) {
	var children []Metadata
	newCursor, err := dbAccess.SelectInt("select max(id) from revisions where user_id = ?", user.Id)
	if err != nil {
		return nil, 0, err
	}
	if _, err := dbAccess.Select(children, `select files.path Path, revisions.name Name, files.is_dir IsDir, revisions.size Size, revisions.id Rev, revisions.modified Modified, files.is_removed IsRemoved
	                                     from files join revisions on files.current_revision_id = revisions.id 
																			 where files.user_id = ? and revisions.id > ?`, user.Id, cursor); err != nil {
		return nil, 0, err
	}
	resp := make([]map[string]interface{}, 0)
	for _, child := range children {
		if child.IsRemoved {
			resp = append(resp, map[string]interface{}{child.Path: nil})
		} else {
			resp = append(resp, map[string]interface{}{child.Path: child})
		}
	}
	return resp, newCursor, nil
}

func (user *User) GetFileByPath(path string) (file *File, err error) {
	file = new(File)
	count, err := dbAccess.SelectInt("select count(*) from files where user_id = ? and path = ?", user.Id, path)
	if err != nil {
		logger.Error(err)
		return nil, err
	}
	if count < 1 {
		return nil, nil
	}
	if err := dbAccess.SelectOne(file, "select * from files where user_id = ? and path = ?", user.Id, path); err != nil {
		logger.Error(err)
		return nil, err
	}
	return file, nil
}

func (user *User) CreateFolder(filepath string) (file *File, err error) {
	logger.Debugf("Creating file entry for folder %s", filepath)
	return user.CreateFile(filepath, true, false, "", 0)
}
func (user *User) CreateFile(filepath string, isDir bool, overwrite bool, uuid string, size int64) (file *File, err error) {
	dir := toolkit.Dir(toolkit.NormalizePath(filepath))
	if dir != "." && dir != "/" {
		file, err := user.GetFileByPath(dir)
		if file != nil {
			if !file.IsDir {
				return nil, errors.New("parent path exists and is not a folder")
			}
		} else {
			if err != nil {
				return nil, err
			} else {
				return nil, errors.New("Parent folder does not exist")
			}
		}
	}
	file, err = user.GetFileByPath(toolkit.NormalizePath(filepath))
	if file != nil && !overwrite {
		if file.IsDir {
			return nil, errors.New("folder already exists")
		} else {
			return nil, errors.New("filepath already exists and is not a folder")
		}
	}
	tx, err := dbAccess.Begin()
	if err != nil {
		return nil, err
	}
	if file == nil {
		file = new(File)
		file.Path = toolkit.NormalizePath(filepath)
		file.Parent = toolkit.Dir(toolkit.NormalizePath(filepath))
		file.IsDir = isDir
		file.IsRemoved = false
		file.UserId = user.Id
		err = tx.Insert(file)
		if err != nil {
			tx.Rollback()
			return nil, err
		}
	} else {
		file.IsDir = isDir
		file.IsRemoved = false
		_, err = tx.Update(file)
		if err != nil {
			tx.Rollback()
			return nil, err
		}
	}
	var revision *Revision = new(Revision)
	revision.Size = size
	revision.IsDir = isDir
	revision.Modified = time.Now()
	revision.Uuid = uuid
	revision.FileId = file.Id
	revision.Name = path.Base(filepath)
	revision.UserId = user.Id
	err = tx.Insert(revision)
	if err != nil {
		tx.Rollback()
		return nil, err
	}
	file.CurrentRevisionId = revision.Id
	_, err = tx.Update(file)
	if err != nil {
		tx.Rollback()
		return nil, err
	}
	// err = user.AddChange(tx, file)
	// if err != nil {
	// 	return nil, err
	// }
	if err = tx.Commit(); err != nil {
		return nil, err
	}
	return file, nil

}

func (user *User) Remove(filepath string) (file *File, err error) {
	file, err = user.GetFileByPath(filepath)
	if err != nil {
		return nil, err
	}
	if file == nil {
		return nil, errors.New("file does not exist")
	}
	tx, err := dbAccess.Begin()
	err = file.Remove(tx)
	if err != nil {
		return nil, err
	}
	if err = tx.Commit(); err != nil {
		return nil, err
	}
	return file, nil

}

// func (user *User) AddChange(tx *gorp.Transaction, file *File) error {
// 	var newCursor int64 = 0
// 	count, err := tx.SelectInt("select count(*) from changes where user_id = ?", user.Id)
// 	if err != nil {
// 		tx.Rollback()
// 		return err
// 	}
// 	if count > 0 {
// 		newCursor, err = tx.SelectInt("select max(cursor_new) from changes where user_id = ?", user.Id)
// 		if err != nil {
// 			tx.Rollback()
// 			return err
// 		}
// 	}
// 	change := Change{FileId: file.Id, UserId: user.Id, CursorOld: newCursor, CursorNew: newCursor + 1}
// 	err = tx.Insert(&change)
// 	if err != nil {
// 		tx.Rollback()
// 		return err
// 	}
// 	return nil
// }

func (user *User) GetRevision(file *File, rev int64) (revision *Revision, err error) {
	revision = new(Revision)
	if err := dbAccess.SelectOne(revision, "select * from revisions where id=? and user_id = ? and file_id=?", rev, user.Id, file.Id); err != nil {
		logger.Error(err)
		return nil, err
	}
	return revision, nil
}

func (user *User) CreateRevision(filepath string, uuidVal string, size int64) (rev *Revision, err error) {
	file, err := user.CreateFile(filepath, false, true, uuidVal, size)
	if err != nil {
		return nil, err
	}
	logger.Debug("Created file: " + filepath)
	rev, err = file.GetCurrentRevision()
	if err != nil {
		return nil, err
	}
	return rev, nil
}
