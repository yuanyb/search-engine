package db

import (
	"database/sql"
	_ "github.com/mattn/go-sqlite3"
)

// TODO documentDb 使用 hbase
type IndexDB struct {
	documentDB *sql.DB
	indexDB    *sql.DB

	getTokenId         *sql.Stmt
	storeToken         *sql.Stmt
	getPostingsList    *sql.Stmt
	updatePostingsList *sql.Stmt
	getDocumentsCount  *sql.Stmt
}

func NewIndexDB(documentDBPath, indexDBPath string) *IndexDB {
	db := new(IndexDB)

	docDB, err := sql.Open("sqlite3", documentDBPath)
	handleDBInitError(err)
	db.documentDB = docDB

	indexDB, err := sql.Open("sqlite3", indexDBPath)
	handleDBInitError(err)
	db.indexDB = indexDB

	// 建表&索引
	_, err = docDB.Exec(`create table if not exists documents(
 		id integer primary key , title text not null, body text not null)`)
	handleDBInitError(err)
	_, err = docDB.Exec(`create unique index title_index on documents(title)`)
	handleDBInitError(err)

	_, err = indexDB.Exec(`create table if not exists tokens(
 		id integer primary key , token text not null,
 		docs_count integer not null, postings blob not null)`)
	handleDBInitError(err)
	_, err = docDB.Exec(`create unique index token_index on tokens(token)`)

	// 初始化语句
	stmt, err := indexDB.Prepare("select id from tokens where token = ?")
	handleDBInitError(err)
	db.getTokenId = stmt

	stmt, err = indexDB.Prepare("insert into tokens values(?, ?, ?)")
	handleDBInitError(err)
	db.storeToken = stmt

	stmt, err = indexDB.Prepare("select postings from token where id = ?")
	handleDBInitError(err)
	db.getPostingsList = stmt

	stmt, err = indexDB.Prepare("update token set postings = ? where id = ?")
	handleDBInitError(err)
	db.updatePostingsList = stmt

	stmt, err = indexDB.Prepare("select count(*) from documents")
	handleDBInitError(err)
	db.getDocumentsCount = stmt

	return db
}

func handleDBInitError(err error) {
	if err != nil {
		panic("启动失败：" + err.Error())
	}
}

func (db *IndexDB) GetTokenId(token string) (int, error) {
	var id int
	err := db.getTokenId.QueryRow(token).Scan(&id)
	return id, err
}

func (db *IndexDB) GetPostingsList(tokenId int) ([]byte, error) {
	var v []byte
	err := db.getPostingsList.QueryRow(tokenId).Scan(&v)
	return v, err
}

func (db *IndexDB) UpdatePostingsList(tokenId int, data []byte) error {
	_, err := db.updatePostingsList.Exec(data, tokenId)
	return err
}

func (db *IndexDB) GetDocumentsCount() (int, error) {
	var count int
	err := db.getDocumentsCount.QueryRow().Scan(&count)
	return count, err
}

func (db *IndexDB) GetDocUrl(id int) string {
	return ""
}
