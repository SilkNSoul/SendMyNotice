package templates

import "embed"

//go:embed notice.html
var NoticeFS embed.FS

// GetNoticeFS exports the embedded filesystem so other packages can use it
func GetNoticeFS() embed.FS {
	return NoticeFS
}