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

var GlobalConfigDB = newConfigDB()

func newConfigDB() *ConfigDB {
	configDB := &ConfigDB{}
	dsn := fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?charset=utf8", "root", "root", "localhost", "3306", "search-engine-config")
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		panic("数据库错误：" + err.Error())
	}
	if err = db.Ping(); err != nil {
		panic("数据库错误：" + err.Error())
	}

	configDB.getConfig, err = db.Prepare("select `value` from `search_engine_indexer` where `name` = ?")
	if err != nil {
		panic("数据库错误：" + err.Error())
	}

	configDB.getIllegalKeywords, err = db.Prepare("select `illegal_keywords` from `search_engine_illegal_keyword` limit 1")
	if err != nil {
		panic("数据库错误：" + err.Error())
	}
	return configDB
}

func (db *ConfigDB) GetConfig(name string) (string, error) {
	var value string
	err := db.getConfig.QueryRow(name).Scan(&value)
	return value, err
}

func (db ConfigDB) GetIllegalKeyWords() ([]string, error) {
	var value string
	err := db.getConfig.QueryRow().Scan(&value)
	ret := strings.Split(strings.TrimSpace(value), "|")
	for i := range ret {
		ret[i] = strings.TrimSpace(ret[i])
	}
	return ret, err
}
