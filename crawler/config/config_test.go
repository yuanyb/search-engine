package config

import (
	"math/rand"
	"testing"
	"time"
)

func TestGet(t *testing.T) {
	ua := []byte("qut_spider")
	rand.Shuffle(len("qut_spider"), func(i, j int) {
		ua[i], ua[j] = ua[j], ua[i]
	})
	stmt, _ := db.Prepare("update `config` set `value` = ? where `name` = 'useragent'")
	_, _ = stmt.Exec(ua)
	time.Sleep(time.Second * 3)
	if Get().Useragent != string(ua) {
		t.Fatal("failed")
	}
}
