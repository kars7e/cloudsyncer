package db

import (
	"cloudsyncer/toolkit"
	"errors"
	"path"
	"time"
)

// Struct describing single user.
type User struct {
	Id       int64  `db:"id"`
	Username string `db:"username"`
	Salt     string `db:"salt"`
	Password string `db:"password"`
}

// Returns true if provided password matches record in database.
func (user *User) CheckPassword(password string) bool {
	if toolkit.GetSha1([]byte(user.Salt+password)) == user.Password {
		return true
	}
	return false
}

// If User with provided username exists in database, returns pointer to user struct. Returns nil otherwise.
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

// Creates user with given username and password. returns created user struct.
// If there was an error when creating user, returns nil and error (for example: User already exists).
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

// Returns all children for given path as a slice of type Metadata.
// Returns double nil if there was no error but no children exist.
// Returns nil and error if error occured.
// This method is defined as *User method to return only user files.
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
	if _, err := dbAccess.Select(&children, `select revisions.hash Hash, revisions.name Name, files.path Path, files.is_dir IsDir, revisions.size Size, revisions.id Rev, revisions.modified Modified
	                                     from files join revisions on files.current_revision_id = revisions.id
																			 where files.user_id = ? and files.is_removed = 0 and files.parent = ? `, user.Id, path); err != nil {
		logger.Error(err)
		return nil, err
	}
	return children, nil

}

// Returns current file state for user, which means all not deleted files and directories currently existing for this user.
// The returned value is a slice of maps, where map has path as key and Metadata as value.
// Returns nil and error if error has occured.
func (user *User) GetCurrentState() ([]map[string]interface{}, error) {
	children, err := user.GetChildren("/") //BUG GetChildren is not recursive, so GetCurrentState() does not return correct value.
	if err != nil {
		return nil, err
	}
	resp := make([]map[string]interface{}, 0)
	for _, child := range children {
		resp = append(resp, map[string]interface{}{child.Path: child})
	}
	return resp, nil
}

//Returns list of changes made since given cursor. Returns empty map if no new changes were made.
//Returns slice of maps, where map has path as key and Metadata as value if file exists, or nil as value if file has been removed since that cursor.
//Returns current cursor if changes were made.
//Returns nil, 0 and error if error has occured.
func (user *User) GetChangesFromCursor(cursor int64) ([]map[string]interface{}, int64, error) {
	var children []Metadata

	counter, err := dbAccess.SelectInt("select count(id) from revisions where user_id = ?", user.Id)
	if err != nil {
		return nil, 0, err
	}
	if counter < 1 {
		resp := make([]map[string]interface{}, 0) //TODO: Change resp to nil, check where it used
		return resp, 0, nil
	}
	newCursor, err := dbAccess.SelectInt("select max(id) from revisions where user_id = ?", user.Id)
	if err != nil {
		return nil, 0, err
	}
	if _, err := dbAccess.Select(&children, `select revisions.hash Hash, files.path Path, revisions.name Name, files.is_dir IsDir, revisions.size Size, revisions.id Rev, revisions.modified Modified, files.is_removed IsRemoved
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

//If file exists, returns pointer to file struct for given path. If file does not exist, returns double nil.
//Returns nil and error if error has occured.
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

//Creates new folder in given path. Uses CreateFile() for this task. If successful, returns pointer to file struct.
//Returns nil and error if error has occured.
func (user *User) CreateFolder(filepath string) (file *File, err error) {
	logger.Debugf("Creating file entry for folder %s", filepath)
	return user.CreateFile(filepath, true, false, "", 0, "")
}

//Creates new file on given path, with specified parameters. All inserts and updates in database are made in single transaction,
// so if error occurs no data is saved. Returns pointer to file struct if successful. Returns nil and error if error has occured.
func (user *User) CreateFile(filepath string, isDir bool, overwrite bool, uuid string, size int64, hash string) (file *File, err error) {
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
	revision.Hash = hash
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
	if err = tx.Commit(); err != nil {
		return nil, err
	}
	return file, nil

}

// Removes file at given filepath. Remove does not delete actual record in database, it only sets value of is_removed to true.
// returns pointer to file struct if successful. Returns nil and error if error has occured.
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

// Creates new revision for given path, with given parameters. Uses CreateFile() for that task.
// If successful, returns pointer to Revision struct. Returns nil and error if error has occured.
func (user *User) CreateRevision(filepath string, uuidVal string, size int64, hash string) (rev *Revision, err error) {
	file, err := user.CreateFile(filepath, false, true, uuidVal, size, hash)
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
