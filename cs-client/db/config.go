package db

// Struct which keeps application configuration
type Config struct {
	Id    int64  `db:"id"`
	Key   string `db:"key"`
	Value string `db:"value"`
}

// Returns value stored for given key. Returns empty string if key does not exist.
func GetCfgValue(key string) (value string) {
	value, err := dbAccess.SelectStr("select value from config where key=?", key)
	if err != nil {
		return ""
	}
	return value
}

// Sets given value for given key. Returns true if there is a success.
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
