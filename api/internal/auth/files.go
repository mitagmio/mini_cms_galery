package auth

import (
	"net/http"
	"os"
	"path"
)

// FileServer serves files from root without directory listings.
// Directories without index.html return 403.
func FileServer(root string) http.Handler {
	return http.FileServer(noListFS{http.Dir(root)})
}

type noListFS struct {
	fs http.FileSystem
}

func (n noListFS) Open(name string) (http.File, error) {
	f, err := n.fs.Open(name)
	if err != nil {
		return nil, err
	}
	st, err := f.Stat()
	if err != nil {
		_ = f.Close()
		return nil, err
	}
	if !st.IsDir() {
		return f, nil
	}
	idx, err := n.fs.Open(path.Join(name, "index.html"))
	if err != nil {
		_ = f.Close()
		return nil, os.ErrPermission
	}
	_ = idx.Close()
	return f, nil
}
