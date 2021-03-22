package scheduler

// Group 表示一个 URL 组，leader 这个 URL 对应页面文档中的所有链接就是 members
type Group struct {
	leader  string
	members []string
}
