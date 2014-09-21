package db

import (
	"time"

	"github.com/nu7hatch/gouuid"
)

// Session struct keeps information about single user session.
type Session struct {
	Id           int64  `db:"id"`
	UserId       int64  `db:"user_id"`
	Token        string `db:"token"`
	Created      int64  `db:"created"`
	ComputerName string `db:"computername"`
}

// Returns user attached with this session. returns nil if there was an error.
func (session *Session) GetUser() *User {
	if session.UserId == 0 {
		logger.Error("GetUser() on session with UserId = 0")
		return nil
	}
	if obj, err := dbAccess.Get(User{}); err != nil {
		logger.Error(err)
		return nil
	} else {
		return obj.(*User)
	}

}

// Creates new session for given user and computer name. Session does not check anything (for example password),
// It just creates new token and stores session record in database. If successful, returns pointer to session struct.
// Returns nil and error if error has occured.
func CreateSession(user *User, computername string) (*Session, error) {
	u4, err := uuid.NewV4()
	var calculatedToken = u4.String()
	var session = Session{UserId: user.Id, Token: calculatedToken, ComputerName: computername, Created: time.Now().Unix()}
	err = dbAccess.Insert(&session)
	return &session, err
}

// Returns session for given user and token. If successful, returns pointer to session struct.
// Returns nil if session does not exist or error has occured.
func GetSession(user *User, tokenString string) *Session {
	var session Session
	if err := dbAccess.SelectOne(&session, "select * from sessions where user_id=? and token=?", user.Id, tokenString); err != nil {
		logger.Error(err)
		return nil
	}
	if session.Token == "" {
		logger.Warning(ErrEntityNotExists)
		return nil

	}
	return &session

}
