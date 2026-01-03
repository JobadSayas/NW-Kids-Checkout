package static

import (
	"embed"
	"io/fs"
	"path"
	"strings"
)

//go:embed *
var EmbeddedFS embed.FS

type filteredFS struct {
	fs      fs.FS
	allowed map[string]struct{}
}

func NewFilteredFS() filteredFS {
	return filteredFS{
		fs:      EmbeddedFS,
		allowed: allowedExt,
	}
}

func (f filteredFS) Open(name string) (fs.File, error) {
	// Normalize path (Fiber may pass leading slash)
	name = strings.TrimPrefix(path.Clean(name), "/")

	// Block directory access
	if strings.HasSuffix(name, "/") || name == "." {
		return nil, fs.ErrNotExist
	}

	ext := strings.ToLower(path.Ext(name))
	if _, ok := f.allowed[ext]; !ok {
		return nil, fs.ErrNotExist
	}

	return f.fs.Open(name)
}

var allowedExt = map[string]struct{}{
	".html": {},
	".css":  {},
	".js":   {},
	".svg":  {},
	".jpg":  {},
	".png":  {},
	".webp": {},
}
