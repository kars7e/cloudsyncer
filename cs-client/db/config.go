package db

type Config struct {
	Id    int64  `db:"id"`
	Key   string `db:"key"`
	Value string `db:"value"`
}

func GetCfgValue(key string) (value string) {
	value, err := dbAccess.SelectStr("select value from config where key=?", key)
	if err != nil {
		return ""
	}
	return value
}

func SetCfgValue(key string, value string) (success bool) {
	var config Config
	dbAccess.SelectOne(&config, "select * from config where key=?", key)
	config.Value = value
	config.Key = key
	if config.Id == 0 {
		if err := dbAccess.Insert(&config); err != nil {
			return false
		}
	} else {
		if _, err := dbAccess.Update(&config); err != nil {
			return false
		}
	}
	return true

}
