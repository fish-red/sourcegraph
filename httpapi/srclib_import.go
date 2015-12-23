package httpapi

import (
	"archive/zip"
	"bytes"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	pathpkg "path"

	"golang.org/x/tools/godoc/vfs"
	"golang.org/x/tools/godoc/vfs/zipfs"

	"strings"

	srclib "sourcegraph.com/sourcegraph/srclib/cli"
	"sourcegraph.com/sourcegraph/srclib/store/pb"
	"src.sourcegraph.com/sourcegraph/util/handlerutil"
	"src.sourcegraph.com/sourcegraph/util/httputil/httpctx"
)

var newSrclibStoreClient = pb.Client // mockable for testing

// serveSrclibImport accepts a zip archive of a .srclib-cache
// directory and runs an import of the data into the repo rev
// specified in the URL route.
func serveSrclibImport(w http.ResponseWriter, r *http.Request) error {
	// Check allowable content types and encodings.
	const allowedContentTypes = "|application/x-zip-compressed|application/x-zip|application/zip|application/octet-stream|"
	if ct := r.Header.Get("content-type"); !strings.Contains(allowedContentTypes, ct) || strings.Contains(ct, "|") {
		http.Error(w, "requires one of Content-Type: "+allowedContentTypes, http.StatusBadRequest)
		return nil
	}
	if strings.ToLower(r.Header.Get("content-transfer-encoding")) != "binary" {
		http.Error(w, "requires Content-Transfer-Encoding: binary", http.StatusBadRequest)
		return nil
	}

	ctx := httpctx.FromRequest(r)
	cl := handlerutil.APIClient(r)

	_, repoRev, _, err := handlerutil.GetRepoAndRev(r, cl.Repos)
	if err != nil {
		return err
	}

	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		return err
	}
	zipR, err := zip.NewReader(bytes.NewReader(body), int64(len(body)))
	if err != nil {
		return err
	}

	// It's safe to construct the zip.ReadCloser without its private
	// *os.File field. If package zipfs's implementation changes in
	// such a way that makes this assumption false, our tests will
	// catch the issue.
	fs := zipfs.New(&zip.ReadCloser{Reader: *zipR}, fmt.Sprintf("srclib import for %s", repoRev))
	fs = absolutePathVFS{fs}

	// Import and index over gRPC.
	remoteStore := newSrclibStoreClient(ctx, pb.NewMultiRepoImporterClient(cl.Conn))

	importOpt := srclib.ImportOpt{
		Repo:     repoRev.URI,
		CommitID: repoRev.CommitID,
		Verbose:  false,
	}
	if err := srclib.Import(fs, remoteStore, importOpt); err != nil {
		return fmt.Errorf("srclib import of %s failed: %s", repoRev, err)
	}

	return nil
}

// absolutePathVFS translates relative paths to paths beginning with
// "/" (which the zipfs VFS requires).
type absolutePathVFS struct {
	vfs.FileSystem
}

func (fs absolutePathVFS) abs(path string) string {
	path = pathpkg.Clean(path)
	switch {
	case path == ".":
		return "/"
	case path[0] == '/':
		return path
	}
	return "/" + path
}

func (fs absolutePathVFS) Stat(path string) (os.FileInfo, error) {
	return fs.FileSystem.Stat(fs.abs(path))
}
func (fs absolutePathVFS) Lstat(path string) (os.FileInfo, error) {
	return fs.FileSystem.Lstat(fs.abs(path))
}
func (fs absolutePathVFS) Open(path string) (vfs.ReadSeekCloser, error) {
	return fs.FileSystem.Open(fs.abs(path))
}
func (fs absolutePathVFS) ReadDir(path string) ([]os.FileInfo, error) {
	return fs.FileSystem.ReadDir(fs.abs(path))
}
func (fs absolutePathVFS) String() string { return fmt.Sprintf("abs(%s)", fs.FileSystem) }