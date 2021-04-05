package db

import (
	"database/sql"
	_ "github.com/mattn/go-sqlite3"
	"sync"
	"sync/atomic"
	"time"
)

// TODO documentDb 使用 hbase
type IndexDB struct {
	documentDB *sql.DB
	indexDB    *sql.DB

	getTokenId        *sql.Stmt
	tokenIdBuffer     *buffer
	getPostings       *sql.Stmt
	postingsBuffer    *buffer
	getDocumentsCount *sql.Stmt
	docCountBuffer    struct {
		lock     sync.Mutex
		birthday int64 // 上次获取文档数量的时间
		count    int64
	}
	getDocumentUrl *sql.Stmt
	docUrlBuffer   *buffer
	storeToken     *sql.Stmt
	updatePostings *sql.Stmt
}

type IndexDBOptions struct {
	docUrlBufferSize   int
	tokenIdBufferSize  int
	postingsBufferSize int
	documentDBPath     string
	indexDBPath        string
}

func NewIndexDB(options *IndexDBOptions) *IndexDB {
	db := new(IndexDB)

	docDB, err := sql.Open("sqlite3", options.documentDBPath)
	handleDBInitError(err)
	db.documentDB = docDB

	indexDB, err := sql.Open("sqlite3", options.indexDBPath)
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
	stmt, err := indexDB.Prepare("select id, docs_count from tokens where token = ?")
	handleDBInitError(err)
	db.getTokenId = stmt
	db.tokenIdBuffer = newBuffer(options.tokenIdBufferSize, func(token interface{}) (interface{}, error) {
		var tokenId, docsCount int
		err := db.getTokenId.QueryRow(token).Scan(&tokenId, &docsCount)
		return [2]int{tokenId, docsCount}, err
	})

	// 获取倒排列表
	stmt, err = indexDB.Prepare("select postings from token where id = ?")
	handleDBInitError(err)
	db.getPostings = stmt
	db.postingsBuffer = newBuffer(options.postingsBufferSize, func(tokenId interface{}) (interface{}, error) {
		var v []byte
		err := db.getPostings.QueryRow(tokenId).Scan(&v)
		return v, err
	})
	// 修改倒排列表
	stmt, err = indexDB.Prepare("replace into tokens(id, postings) values(?, ?)")
	handleDBInitError(err)
	db.updatePostings = stmt
	// 文档数量
	stmt, err = indexDB.Prepare("select count(*) from documents")
	handleDBInitError(err)
	db.getDocumentsCount = stmt
	// 文档URL
	stmt, err = indexDB.Prepare("select url from documents where id = ?")
	handleDBInitError(err)
	db.getDocumentUrl = stmt
	db.docUrlBuffer = newBuffer(options.docUrlBufferSize, func(docId interface{}) (interface{}, error) {
		var url string
		err := db.getDocumentUrl.QueryRow(docId).Scan(&url)
		return url, err
	})

	return db
}

func handleDBInitError(err error) {
	if err != nil {
		panic("启动失败：" + err.Error())
	}
}

// 根据词元获取id
func (db *IndexDB) GetTokenId(token string) (int, int, error) {
	pair, err := db.tokenIdBuffer.get(token)
	arr := pair.([2]int) // tokenId,docsCount
	return arr[0], arr[1], err
}

// 获取词元的倒排列表
func (db *IndexDB) GetPostings(tokenId int) ([]byte, error) {
	postingsList, err := db.postingsBuffer.get(tokenId)
	return postingsList.([]byte), err
}

// 修改指定词元的倒排列表
func (db *IndexDB) UpdatePostings(tokenId int, data []byte) error {
	_, err := db.updatePostings.Exec(tokenId, data, data)
	return err
}

// 获取文档数量
func (db *IndexDB) GetDocumentsCount() (int, error) {
	// 10 秒有效期
	if time.Now().Unix()-atomic.LoadInt64(&db.docCountBuffer.birthday) < 10 {
		return int(atomic.LoadInt64(&db.docCountBuffer.count)), nil
	}

	db.docCountBuffer.lock.Lock()
	defer db.docCountBuffer.lock.Unlock()
	// 双重检查
	if time.Now().Unix()-db.docCountBuffer.birthday < 10 {
		return int(db.docCountBuffer.count), nil
	}
	var count int
	err := db.getDocumentsCount.QueryRow().Scan(&count)
	db.docCountBuffer.birthday = time.Now().Unix()
	return count, err
}

// 根据文档 id 获取文档 url
func (db *IndexDB) GetDocUrl(id int) (string, error) {
	url, err := db.docUrlBuffer.get(id)
	return url.(string), err
}
