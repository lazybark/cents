package server

import (
	"log/slog"
	"net/http"
	"strconv"

	"github.com/lazybark/cents/logview"
)

func adminLogs(path string, log *slog.Logger) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			w.Header().Set("Allow", http.MethodGet)
			writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "use GET")
			return
		}
		values := r.URL.Query()
		q := logview.Query{Level: values.Get("level"), Component: values.Get("component"), Client: values.Get("client"), Status: values.Get("status"), Search: values.Get("search"), From: values.Get("from"), To: values.Get("to")}
		var err error
		if raw := values.Get("limit"); raw != "" {
			q.Limit, err = strconv.Atoi(raw)
		}
		if err != nil {
			writeError(w, 400, "bad_request", "limit must be a number")
			return
		}
		if raw := values.Get("beforeId"); raw != "" {
			q.BeforeID, err = strconv.ParseInt(raw, 10, 64)
		}
		if err != nil {
			writeError(w, 400, "bad_request", "beforeId must be a number")
			return
		}
		q, err = q.Normalize()
		if err != nil {
			writeError(w, 400, "bad_request", err.Error())
			return
		}
		page, err := logview.Read(r.Context(), path, q)
		if err != nil {
			log.Error("reading administration logs failed", "error", err, "client", info(r).client)
			writeError(w, 503, "logs_unavailable", "the logs database is unavailable; check the server log")
			return
		}
		writeJSON(w, http.StatusOK, page)
	})
}
