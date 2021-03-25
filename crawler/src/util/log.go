// 为了使用方便 & 可以实时修改日志级别，对标准库日志包做一层封装
package util

import (
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
)

var Logger = NewLogger(DefaultLogFlags)

type Log struct {
	mutex  sync.Mutex
	logger *log.Logger
	_level int32
	file   *os.File
}

// 日志级别，自上向下级别越来越低
const DefaultLogFlags = log.Ldate | log.Lmicroseconds | log.Llongfile
const (
	LDebug = iota
	LInfo
	LWaring
	LError
	LFatal
	LOff
)

func (l *Log) SetLevel(level int) {
	atomic.StoreInt32(&l._level, int32(level))
}

func (l *Log) GetLevel() int {
	return int(atomic.LoadInt32(&l._level))
}

func (l *Log) Debug(format string, v ...interface{}) {
	if l.GetLevel() > LDebug {
		return
	}
	l.doLog("DEBUG: ", format, v)
}

func (l *Log) Info(format string, v ...interface{}) {
	if l.GetLevel() > LInfo {
		return
	}
	l.doLog("INFO: ", format, v)
}

func (l *Log) Warning(format string, v ...interface{}) {
	if l.GetLevel() > LWaring {
		return
	}
	l.doLog("WARNING: ", format, v)
}

func (l *Log) Error(format string, v ...interface{}) {
	if l.GetLevel() > LError {
		return
	}
	l.doLog("ERROR: ", format, v)
}

func (l *Log) Fatal(format string, v ...interface{}) {
	if l.GetLevel() > LFatal {
		return
	}
	l.doLog("FATAL: ", format, v)
}

func (l *Log) doLog(prefix, format string, v []interface{}) {
	l.logger.SetPrefix(prefix)
	if len(v) > 0 {
		format = fmt.Sprintf(format, v...)
	}
	_ = l.logger.Output(3, format)
	l.checkRollback()
}

var checkCount int32

// 如果大于 50MB 或到了新的一天，则回滚
// 为了性能每调用 20 次检查一次
func (l *Log) checkRollback() {
	l.mutex.Lock()
	defer l.mutex.Unlock()
	if checkCount < 20 {
		checkCount++
		return
	} else {
		checkCount = 0
	}
	stat, err := l.file.Stat()
	if err != nil {
		return
	}
	if stat.Size() < 50*1024*1024 {
		return
	}
	s := strings.Split(strings.Split("@", stat.Name())[1], "_")
	date, no := s[0], s[1]
	if Today() == date {
		return
	}
	nextNo, _ := strconv.Atoi(no)
	nextNo++
	file, err := os.OpenFile(fmt.Sprintf("./crawler@%s_%2d.log", Today(), nextNo),
		os.O_CREATE|os.O_APPEND, 0666)
	if err != nil {
		l.Fatal(fmt.Sprintf("打开日志文件[crawler@%s_%2d.log]失败", Today(), nextNo))
		return
	}
	_ = l.file.Close()
	l.file = file
	l.logger.SetOutput(file)
}

func NewLogger(flags int) *Log {
	file, err := os.OpenFile(fmt.Sprintf("./crawer@%s_01.log", Today()),
		os.O_CREATE|os.O_APPEND, 0666)
	if err != nil {
		panic(fmt.Sprintf("打开日志文件[./crawer@%s_01.log]失败", Today()))
	}
	l := &Log{
		logger: log.New(file, "", flags),
		_level: LInfo,
		file:   file,
	}
	return l
}
