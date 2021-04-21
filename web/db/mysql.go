package db

import (
	"database/sql"
	"fmt"
	_ "github.com/go-sql-driver/mysql"
	"log"
	"search-engine/web/config"
	"strings"
)

type MysqlDB struct {
	db                  *sql.DB
	getIllegalKeywords  *sql.Stmt
	addIllegalKeywords  *sql.Stmt
	delIllegalKeyword   *sql.Stmt
	getDomainBlackList  *sql.Stmt
	delDomain           *sql.Stmt
	login               *sql.Stmt
	updateCrawlerConfig *sql.Stmt
	getCrawlerConfig    *sql.Stmt
}

type MysqlDBOptions struct {
	User     string
	Password string
	Host     string
	Port     int
	DBName   string
}

var Mysql = NewMysqlDB(&MysqlDBOptions{
	User:     config.Get("mysql.username"),
	Password: config.Get("mysql.password"),
	Host:     config.Get("mysql.host"),
	Port:     config.GetInt("mysql.port"),
	DBName:   config.Get("mysql.dbname"),
})

func NewMysqlDB(options *MysqlDBOptions) *MysqlDB {
	mysqlDB := &MysqlDB{}
	dsn := fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?charset=utf8",
		options.User, options.Password, options.Host, options.Port, options.DBName)
	checkDBInitError := func(err error) {
		if err != nil {
			log.Fatalln("初始化Mysql失败", err.Error())
		}
	}
	db, err := sql.Open("mysql", dsn)
	checkDBInitError(err)
	err = db.Ping()
	checkDBInitError(err)

	mysqlDB.getIllegalKeywords, err = db.Prepare("select `keyword` from `illegal_keyword`")
	checkDBInitError(err)

	mysqlDB.delIllegalKeyword, err = db.Prepare("delete from `illegal_keyword` where `keyword` = ?")
	checkDBInitError(err)

	mysqlDB.getDomainBlackList, err = db.Prepare("select * from `domain_blacklist`")
	checkDBInitError(err)

	mysqlDB.delDomain, err = db.Prepare("delete from `domain_blacklist` where `domain` = ?")
	checkDBInitError(err)

	mysqlDB.login, err = db.Prepare("select count(*) > 0 from admin where `username` = ? and `password` = ?")
	checkDBInitError(err)

	//
	mysqlDB.updateCrawlerConfig, err = db.Prepare("replace into crawler(name, value) values(?, ?)")
	checkDBInitError(err)

	mysqlDB.getCrawlerConfig, err = db.Prepare("select `name`, `value` from `crawler`")
	checkDBInitError(err)
	return mysqlDB
}

func (db *MysqlDB) GetIllegalKeyWords() ([]string, error) {
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

func (db *MysqlDB) AddIllegalKeywords(keywords []string) error {
	sqlBuf := new(strings.Builder)
	sqlBuf.WriteString("insert into `illegal_keyword`(keyword) values")
	for i, kw := range keywords {
		if i < len(keywords)-1 {
			sqlBuf.WriteString(fmt.Sprintf("(%s),", kw))
		} else {
			sqlBuf.WriteString(fmt.Sprintf("(%s);", kw))
		}
	}
	_, err := db.db.Exec(sqlBuf.String())
	return err
}

func (db *MysqlDB) DelIllegalKeyword(keyword string) error {
	_, err := db.delIllegalKeyword.Exec(keyword)
	return err
}

func (db *MysqlDB) GetDomainBlacklist() ([]string, error) {
	rows, err := db.getDomainBlackList.Query()
	if err != nil {
		return nil, err
	}
	var domainList []string
	var domain string
	for rows.Next() {
		err = rows.Scan(&domain)
		if err != nil {
			return nil, err
		}
		domainList = append(domainList, domain)
	}
	return domainList, nil
}

func (db *MysqlDB) AddDomainBlacklist(domainList []string) error {
	sqlBuf := new(strings.Builder)
	sqlBuf.WriteString("insert into `domain_blacklist`(domain) values")
	for i, domain := range domainList {
		if i < len(domainList)-1 {
			sqlBuf.WriteString(fmt.Sprintf("(%s),", domain))
		} else {
			sqlBuf.WriteString(fmt.Sprintf("(%s);", domain))
		}
	}
	_, err := db.db.Exec(sqlBuf.String())
	return err
}
func (db *MysqlDB) DelDomainBlacklist(domain string) error {
	_, err := db.delDomain.Exec(domain)
	return err
}

func (db *MysqlDB) Login(username, password string) (bool, error) {
	row, err := db.login.Query(username, password)
	if err != nil {
		return false, err
	}
	b := false
	row.Next()
	if err = row.Scan(&b); err != nil {
		return false, err
	}
	return b, nil
}

func (db *MysqlDB) UpdateCrawlerConfig(name, value string) error {
	_, err := db.updateCrawlerConfig.Exec(name, value)
	return err
}

func (db *MysqlDB) GetCrawlerConfig() (map[string]string, error) {
	conf := make(map[string]string)
	rows, err := db.getCrawlerConfig.Query()
	if err != nil {
		return nil, err
	}
	var name, value string
	for rows.Next() {
		err = rows.Scan(&name, &value)
		if err != nil {
			return nil, err
		}
		conf[name] = value
	}
	return conf, nil
}
