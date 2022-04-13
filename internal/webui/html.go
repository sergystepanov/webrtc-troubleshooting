package webui

import (
	"embed"
	"io/fs"
	"net/http"
)

//go:embed client
var web embed.FS

// Index handles the web content root content (index page)
func Index() (http.Handler, error) {
	content, err := fs.Sub(fs.FS(web), "client")
	if err != nil {
		return nil, err
	}
	return http.FileServer(http.FS(content)), nil
}