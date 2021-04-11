package db

import (
	"database/sql"
	"fmt"
	_ "github.com/go-sql-driver/mysql"
	"log"
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

	configDB.getIllegalKeywords, err = db.Prepare("select `illegal_keyword` from `search_engine_illegal_keyword`")
	if err != nil {
		panic("数据库错误：" + err.Error())
	}
	return configDB
}

func (db *ConfigDB) GetConfig() (map[string]string, error) {
	conf := make(map[string]string)
	rows, err := db.getConfig.Query()
	if err != nil {
		log.Println(err.Error())
		return conf, err
	}
	var name, value string
	for rows.Next() {
		err = rows.Scan(&name, &value)
		if err != nil {
			log.Println(err.Error())
			return conf, err
		}
		conf[name] = value
	}
	return conf, err
}

func (db ConfigDB) GetIllegalKeyWords() ([]string, error) {
	var illegalKeywords []string
	var w string
	rows, err := db.getIllegalKeywords.Query()
	if err != nil {
		log.Println(err.Error())
		return nil, err
	}
	for rows.Next() {
		err = rows.Scan(&w)
		if err != nil {
			log.Println(err.Error())
			return nil, err
		}
		illegalKeywords = append(illegalKeywords, w)
	}
	return illegalKeywords, err
}
