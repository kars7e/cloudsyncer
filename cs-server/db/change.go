package db

type Change struct {
	Id        int64 `db:"id"`
	FileId    int64 `db:"file_id"`
	CursorOld int64 `db:"cursor_old"`
	CursorNew int64 `db:"cursor_new"`
	UserId    int64 `db:"user_id"`
}
