package db

import (
	"time"

	"github.com/nu7hatch/gouuid"
)

type Session struct {
	Id           int64  `db:"id"`
	UserId       int64  `db:"user_id"`
	Token        string `db:"token"`
	Created      int64  `db:"created"`
	ComputerName string `db:"computername"`
}

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

func CreateSession(user *User, computername string) (*Session, error) {
	u4, err := uuid.NewV4()
	var calculatedToken = u4.String()
	var session = Session{UserId: user.Id, Token: calculatedToken, ComputerName: computername, Created: time.Now().Unix()}
	err = dbAccess.Insert(&session)
	return &session, err
}

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
