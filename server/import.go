package server

import (
	"errors"
	"mime"
	"net/http"
	"reflect"

	"github.com/lazybark/cents/dbimport"
	"github.com/lazybark/cents/service"
)

func importRoute(files service.Files) route {
	return route{method: http.MethodPost, path: "/import", name: "importDatabase", summary: "Merge a cents .db snapshot; preview=true checks without saving. Existing records win conflicts.", query: []string{"preview:bool"}, out: reflect.TypeFor[dbimport.Result](), status: http.StatusOK, requestContentType: "application/vnd.sqlite3", serve: func(_ http.ResponseWriter, r *http.Request, p *params) (any, error) {
		kind, _, _ := mime.ParseMediaType(r.Header.Get("Content-Type"))
		if kind != "application/vnd.sqlite3" && kind != "application/octet-stream" {
			return nil, badRequest{"upload a cents .db file as application/vnd.sqlite3"}
		}
		importer, ok := files.(service.DatabaseImporter)
		if !ok {
			return nil, errors.New("this backend does not support database imports")
		}
		result, err := importer.ImportDatabase(r.Context(), r.Body, p.queryBool(r, "preview", true))
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) {
			err = dbimport.ErrTooLarge
		}
		return result, err
	}}
}

// BodyLimit retains the normal JSON limit, allowing larger binary imports only
// on their exact endpoint. Call before adding MaxBytesReader at each proxy hop.
func BodyLimit(method, path string) int64 {
	if method == http.MethodPost && path == apiPrefix+"/import" {
		return dbimport.MaxBytes
	}
	return maxBody
}
