package config

type Config struct {
	DocUrlBufferSize     int
	TokenIdBufferSize    int
	PostingsBufferSize   int
	DocumentDBPath       string
	IndexDBPath          string
	TokenN               int
	IndexerChannelLength int
	IndexWorkerCount     int
}

var GlobalConfig = &Config{
	DocUrlBufferSize:     0,
	TokenIdBufferSize:    0,
	PostingsBufferSize:   0,
	DocumentDBPath:       "",
	IndexDBPath:          "",
	TokenN:               0,
	IndexerChannelLength: 0,
	IndexWorkerCount:     0,
}

func init() {

}
