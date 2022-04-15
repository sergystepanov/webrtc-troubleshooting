package webui

import (
	"embed"
	"io/fs"
	"net/http"
	"os"
)

//go:embed client
var web embed.FS

// Index handles the web content root content (index page)
func Index(live bool) (http.Handler, error) {
	var content fs.FS
	if live {
		content = os.DirFS("./internal/webui/client")
	} else {
		c, err := fs.Sub(fs.FS(web), "client")
		if err != nil {
			return nil, err
		}
		content = c
	}
	return http.FileServer(http.FS(content)), nil
}
