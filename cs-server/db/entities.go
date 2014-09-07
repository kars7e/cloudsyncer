package db










type Changeset struct {
	Hash string `db:"hash"`
	Parent string `db:"parent"`
	UserId int64 `db:"user_id"`
	Created  int64  `db:"created"`	
}







