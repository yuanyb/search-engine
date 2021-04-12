package db

import (
	"encoding/binary"
	"fmt"
	"github.com/boltdb/bolt"
	"log"
	"search-engine/index/util"
	"sync"
	"sync/atomic"
	"time"
)

var (
	BucketDocUrl        = []byte("doc_url")
	BucketDocDetail     = []byte("doc_detail")
	BucketTokenPostings = []byte("token_postings")
	BucketTokenDocCount = []byte("token_doc_count")
)

type IndexDB struct {
	indexDB *bolt.DB
	docDB   *bolt.DB

	postingsBuffer  *util.Buffer
	docsCountBuffer struct {
		count    int64
		birthday int64
		sync.Mutex
	}
	docUrlBuffer *util.Buffer
}

type IndexDBOptions struct {
	DocUrlBufferSize   int
	PostingsBufferSize int
	DocumentDBPath     string
	IndexDBPath        string
}

func NewIndexDB(options *IndexDBOptions) *IndexDB {
	// 打开数据库
	docDB, err := bolt.Open(options.DocumentDBPath, 0600, nil)
	if err != nil {
		log.Fatalln(err.Error())
	}
	indexDB, err := bolt.Open(options.IndexDBPath, 0600, nil)
	if err != nil {
		_ = docDB.Close()
		log.Fatalln(err.Error())
	}

	// 创建 Bucket
	err = docDB.Update(func(tx *bolt.Tx) error {
		if _, err := tx.CreateBucketIfNotExists(BucketDocUrl); err != nil {
			return err
		}
		_, err = tx.CreateBucketIfNotExists(BucketDocDetail)
		return err
	})
	if err != nil {
		_ = docDB.Close()
		_ = indexDB.Close()
		log.Fatalln(err.Error())
	}
	err = indexDB.Update(func(tx *bolt.Tx) error {
		if _, err := tx.CreateBucketIfNotExists(BucketTokenDocCount); err != nil {
			return err
		}
		_, err := tx.CreateBucketIfNotExists(BucketTokenPostings)
		return err
	})
	if err != nil {
		_ = docDB.Close()
		_ = indexDB.Close()
		log.Fatalln(err.Error())
	}

	// Buffer
	postingsBuffer := util.NewBuffer(options.PostingsBufferSize, func(key interface{}) interface{} {
		var value []byte
		_ = indexDB.View(func(tx *bolt.Tx) error {
			bucket := tx.Bucket(BucketTokenPostings)
			ret := bucket.Get([]byte(key.(string)))
			value = append(ret) // ret 仅在事务期间有效
			return nil
		})
		return value
	})
	docUrlBuffer := util.NewBuffer(options.DocUrlBufferSize, func(key interface{}) interface{} {
		var value string
		_ = docDB.View(func(tx *bolt.Tx) error {
			bucket := tx.Bucket(BucketDocUrl)
			ret := bucket.Get([]byte(fmt.Sprint(key)))
			value = string(ret)
			return nil
		})
		return value
	})

	return &IndexDB{
		docDB:          docDB,
		indexDB:        indexDB,
		postingsBuffer: postingsBuffer,
		docUrlBuffer:   docUrlBuffer,
	}
}

// 构建索引用
func (db *IndexDB) UpdatePostings(fn func(tx *bolt.Tx) error) {
	_ = db.indexDB.Update(fn)
}

// 检索使用
func (db *IndexDB) FetchPostings(token string) []byte {
	return db.postingsBuffer.Get(token).([]byte)
}

// 获取文档库中文档的数量
func (db *IndexDB) GetDocumentsCount() int {
	// 5 秒有效期
	if time.Now().Unix()-atomic.LoadInt64(&db.docsCountBuffer.birthday) < 5 {
		return int(atomic.LoadInt64(&db.docsCountBuffer.count))
	}

	db.docsCountBuffer.Lock()
	defer db.docsCountBuffer.Unlock()

	// 双重检查
	if time.Now().Unix()-db.docsCountBuffer.birthday < 5 {
		return int(db.docsCountBuffer.count)
	}

	var count int
	_ = db.docDB.View(func(tx *bolt.Tx) error {
		count = tx.Bucket(BucketDocDetail).Stats().KeyN
		return nil
	})
	db.docsCountBuffer.birthday = time.Now().Unix()
	db.docsCountBuffer.count = int64(count)
	return count
}

// 根据文档 ID 获取文档的 URL
func (db *IndexDB) GetDocumentUrl(docId int) string {
	return db.docUrlBuffer.Get(docId).(string)
}

func (db *IndexDB) AddDocument(url, title, body string) (int, error) {
	var docId uint64
	err := db.docDB.Update(func(tx *bolt.Tx) error {
		bucketUrl := tx.Bucket(BucketDocUrl)
		bucketDetail := tx.Bucket(BucketDocDetail)
		docId, _ = bucketDetail.NextSequence()
		if err := bucketUrl.Put([]byte(fmt.Sprint(docId)), []byte(url)); err != nil {
			return err
		}

		t := util.EncodeVarInt(make([]byte, binary.MaxVarintLen64), int64(len(title))) // title长度的字节数组
		data := make([]byte, len(t)+len(title)+len(body))
		copy(data, t)
		copy(data[len(t):], title)
		copy(data[len(t)+len(title):], body)
		if err := bucketDetail.Put([]byte(fmt.Sprint(docId)), data); err != nil {
			return err
		}
		return nil
	})
	return int(docId), err
}

func (db *IndexDB) GetDocument(docId int) (string, string, string) {
	var url, title, body string
	_ = db.docDB.View(func(tx *bolt.Tx) error {
		bucketUrl := tx.Bucket(BucketDocUrl)
		bucketDetail := tx.Bucket(BucketDocDetail)
		key := []byte(fmt.Sprint(docId))

		url = string(bucketUrl.Get(key))
		data := bucketDetail.Get(key)
		titleLen, length := binary.Varint(data)
		data = data[length:]
		title = string(data[:titleLen])
		body = string(data[titleLen:])
		return nil
	})
	return url, title, body
}
