package db

import (
	"database/sql"
	_ "github.com/mattn/go-sqlite3"
	"log"
	"search-engine/index/util"
	"sync"
	"sync/atomic"
	"time"
)

// TODO documentDb 使用 hbase
type IndexDB struct {
	documentDB *sql.DB
	indexDB    *sql.DB

	getTokenId        *sql.Stmt
	addToken          *sql.Stmt
	tokenIdBuffer     *util.Buffer
	getPostings       *sql.Stmt
	postingsBuffer    *util.Buffer
	getDocumentsCount *sql.Stmt
	docCountBuffer    struct {
		lock     sync.Mutex
		birthday int64 // 上次获取文档数量的时间
		count    int64
	}
	getDocumentUrl    *sql.Stmt
	docUrlBuffer      *util.Buffer
	getDocumentDetail *sql.Stmt
	storeToken        *sql.Stmt
	updatePostings    *sql.Stmt
	addDocument       *sql.Stmt
}

type IndexDBOptions struct {
	DocUrlBufferSize   int
	TokenIdBufferSize  int
	PostingsBufferSize int
	DocumentDBPath     string
	IndexDBPath        string
}

func NewIndexDB(options *IndexDBOptions) *IndexDB {
	db := new(IndexDB)

	docDB, err := sql.Open("sqlite3", options.DocumentDBPath)
	handleDBInitError(err)
	db.documentDB = docDB

	indexDB, err := sql.Open("sqlite3", options.IndexDBPath)
	handleDBInitError(err)
	db.indexDB = indexDB

	// 建表&索引
	_, err = docDB.Exec(`create table if not exists documents(
 		id integer primary key, url text not null , title text not null, body text not null)`)
	handleDBInitError(err)

	_, err = indexDB.Exec(`create table if not exists tokens(
 		id integer primary key , token text not null,
 		docs_count integer not null, postings blob not null)`)
	handleDBInitError(err)
	_, err = docDB.Exec(`create unique index token_index on tokens(token)`)

	// 查询token
	stmt, err := indexDB.Prepare("select id, docs_count from tokens where token = ?")
	handleDBInitError(err)
	db.getTokenId = stmt
	db.tokenIdBuffer = util.NewBuffer(options.TokenIdBufferSize, func(token interface{}) (interface{}, error) {
		var tokenId, docsCount int
		err := db.getTokenId.QueryRow(token).Scan(&tokenId, &docsCount)
		// 该词元不存在
		if err == sql.ErrNoRows {
			res, err2 := db.addToken.Exec(token)
			if err2 != nil {
				return nil, err2
			}
			id, err2 := res.LastInsertId()
			if err2 != nil {
				return nil, err2
			}
			tokenId = int(id)
		}
		return [2]int{tokenId, docsCount}, nil
	})

	stmt, err = indexDB.Prepare("insert into tokens(token) values(?)")
	handleDBInitError(err)
	db.addToken = stmt

	// 获取倒排列表
	stmt, err = indexDB.Prepare("select postings from tokens where id = ?")
	handleDBInitError(err)
	db.getPostings = stmt
	db.postingsBuffer = util.NewBuffer(options.PostingsBufferSize, func(tokenId interface{}) (interface{}, error) {
		var v []byte
		err := db.getPostings.QueryRow(tokenId).Scan(&v)
		return v, err
	})
	// 修改倒排列表

	// 之前已经查如果token了，但 token 设置为了 not null，如果不提供一个token就会语法错误
	stmt, err = indexDB.Prepare(`insert into tokens(id, token, docs_count, postings)
	 values(?, "", ?, ?) on conflict(id) do update set docs_count=?, postings=?`)
	handleDBInitError(err)
	db.updatePostings = stmt
	// 文档数量
	stmt, err = docDB.Prepare("select count(*) from documents")
	handleDBInitError(err)
	db.getDocumentsCount = stmt
	// 文档URL
	stmt, err = docDB.Prepare("select url from documents where id = ?")
	handleDBInitError(err)
	db.getDocumentUrl = stmt
	db.docUrlBuffer = util.NewBuffer(options.DocUrlBufferSize, func(docId interface{}) (interface{}, error) {
		var url string
		err := db.getDocumentUrl.QueryRow(docId).Scan(&url)
		return url, err
	})
	// 添加文档
	stmt, err = docDB.Prepare("insert into documents(url, title, body) values(?,?,?)")
	handleDBInitError(err)
	db.addDocument = stmt
	// 标题、摘要
	stmt, err = docDB.Prepare("select url, title, body from documents where id = ?")
	handleDBInitError(err)
	db.getDocumentDetail = stmt

	return db
}

func handleDBInitError(err error) {
	if err != nil {
		panic("启动失败：" + err.Error())
	}
}

// 根据词元获取id
func (db *IndexDB) GetTokenId(token string) (int, int, error) {
	pair, err := db.tokenIdBuffer.Get(token)
	if err != nil {
		log.Print(err.Error())
		return 0, 0, err
	}
	arr := pair.([2]int) // tokenId,docsCount
	return arr[0], arr[1], nil
}

// 获取词元的倒排列表
func (db *IndexDB) GetPostings(tokenId int) ([]byte, error) {
	postingsList, err := db.postingsBuffer.Get(tokenId)
	return postingsList.([]byte), err
}

// 修改指定词元的倒排列表
func (db *IndexDB) UpdatePostings(tokenId, docsCount int, data []byte) error {
	_, err := db.updatePostings.Exec(tokenId, docsCount, data, docsCount, data)
	db.postingsBuffer.Del(tokenId) // 删除旧的
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
	url, err := db.docUrlBuffer.Get(id)
	return url.(string), err
}

func (db *IndexDB) AddDocument(url, title, body string) (int, error) {
	res, err := db.addDocument.Exec(url, title, body)
	if err != nil {
		log.Print(err.Error())
		return 0, err
	}
	docId, err := res.LastInsertId()
	return int(docId), err
}

func (db *IndexDB) GetDocumentDetail(docId int) (string, string, string, error) {
	var url, title, abstract string
	// sqlite substr 起始位置是1
	err := db.getDocumentDetail.QueryRow(docId).Scan(&url, &title, &abstract)
	return url, title, abstract, err
}
