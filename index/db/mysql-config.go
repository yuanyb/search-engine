package db

import (
	"database/sql"
	"fmt"
	_ "github.com/go-sql-driver/mysql"
	"strings"
)

type ConfigDB struct {
	db                 *sql.DB
	getConfig          *sql.Stmt
	getIllegalKeywords *sql.Stmt
}

type ConfigDBOptions struct {
	User     string
	Password string
	Host     string
	Port     int
	DBName   string
}

func NewConfigDB(options *ConfigDBOptions) *ConfigDB {
	configDB := &ConfigDB{}
	dsn := fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?charset=utf8",
		options.User, options.Password, options.Host, options.Port, options.DBName)
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		panic("数据库错误：" + err.Error())
	}
	if err = db.Ping(); err != nil {
		panic("数据库错误：" + err.Error())
	}

	configDB.getConfig, err = db.Prepare("select * from `search_engine_indexer`")
	if err != nil {
		panic("数据库错误：" + err.Error())
	}

	configDB.getIllegalKeywords, err = db.Prepare("select `illegal_keywords` from `search_engine_illegal_keyword` limit 1")
	if err != nil {
		panic("数据库错误：" + err.Error())
	}
	return configDB
}

func (db *ConfigDB) GetConfig() (map[string]string, error) {
	conf := make(map[string]string)
	rows, err := db.getConfig.Query()
	if err != nil {
		return conf, err
	}
	var name, value string
	for rows.Next() {
		err = rows.Scan(&name, &value)
		if err != nil {
			return conf, err
		}
		conf[name] = value
	}
	return conf, err
}

func (db ConfigDB) GetIllegalKeyWords() ([]string, error) {
	var value string
	err := db.getIllegalKeywords.QueryRow().Scan(&value)
	ret := strings.Split(strings.TrimSpace(value), "|")
	for i := range ret {
		ret[i] = strings.TrimSpace(ret[i])
	}
	return ret, err
}
