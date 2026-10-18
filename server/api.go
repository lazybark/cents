package server

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"mime"
	"net/http"
	"net/url"
	"reflect"
	"strconv"
	"strings"
	"sync"

	"github.com/lazybark/cents/service"
	storage "github.com/lazybark/cents/storage/sqlite"
)

// Version is the server's version, reported by /healthz.
const Version = "0.2.0"

// apiPrefix is where the API lives; a new major version gets a new one.
const apiPrefix = "/api/v1"

// none is no body, or no result.
type none struct{}

// route is one endpoint: what it is (for routing and the API description)
// and how it's served.
type route struct {
	// name is the operation's name in the API's description, which code
	// generators name their methods after.
	name                  string
	method, path, summary string
	// query lists the query parameters: "name" is text, "name:int" and
	// "name:bool" are checked before the call.
	query []string
	// in is the JSON body read and out the JSON result; nil for none.
	in, out reflect.Type
	// status is the status of success; contentType is set for a file sent
	// instead of JSON.
	status      int
	contentType string
	// optionalBody accepts a request without a body.
	optionalBody bool
	serve        func(w http.ResponseWriter, r *http.Request, p *params) (any, error)
}

func typeOf[T any]() reflect.Type {
	if t := reflect.TypeFor[T](); t != reflect.TypeFor[none]() {
		return t
	}

	return nil
}

// get is a route that reads.
func get[Out any](path, summary string, query []string, call func(*http.Request, *params) (Out, error)) route {
	return route{method: http.MethodGet, path: path, summary: summary, query: query, out: typeOf[Out](), status: http.StatusOK,
		serve: func(_ http.ResponseWriter, r *http.Request, p *params) (any, error) { return call(r, p) }}
}

// send is a route that takes In as its JSON body and answers with Out.
func send[In, Out any](method, path, summary string, call func(*http.Request, *params, In) (Out, error)) route {
	rt := route{method: method, path: path, summary: summary, in: typeOf[In](), out: typeOf[Out](), status: http.StatusOK}
	rt.serve = func(_ http.ResponseWriter, r *http.Request, p *params) (any, error) {
		var in In
		if rt.in != nil {
			if err := decode(r, &in, rt.optionalBody); err != nil {
				return nil, err
			}
		}

		return call(r, p, in)
	}

	return rt
}

// act is a route with neither body nor result.
func act(method, path, summary string, call func(*http.Request, *params) error) route {
	return route{method: method, path: path, summary: summary, status: http.StatusNoContent,
		serve: func(_ http.ResponseWriter, r *http.Request, p *params) (any, error) { return nil, call(r, p) }}
}

// named names the operation.
func (rt route) named(name string) route {
	rt.name = name
	return rt
}

// created answers success with 201 Created.
func (rt route) created() route {
	rt.status = http.StatusCreated
	return rt
}

// noBody reads no body (In is none).
func (rt route) noBody() route {
	rt.in = nil
	return rt
}

// payload collects a file to send: kept whole, so a failure halfway is
// still answered with an error instead of half a file.
type payload struct {
	bytes.Buffer
	headers http.Header
}

func (d *payload) header(name string, value any) {
	d.headers.Set(name, fmt.Sprint(value))
}

func (d *payload) attachment(name string) {
	d.headers.Set("Content-Disposition", mime.FormatMediaType("attachment", map[string]string{"filename": name}))
}

// file is a downloaded file, ready to send.
type file struct{ *payload }

// downloadRoute is a route that sends a file.
func downloadRoute(method, path, summary string, query []string, contentType string, in reflect.Type, write func(d *payload, r *http.Request, p *params) error) route {
	return route{method: method, path: path, summary: summary, query: query, in: in, status: http.StatusOK, contentType: contentType,
		serve: func(_ http.ResponseWriter, r *http.Request, p *params) (any, error) {
			d := &payload{headers: http.Header{}}
			if err := write(d, r, p); err != nil {
				return nil, err
			}

			d.headers.Set("Content-Type", contentType)
			return file{d}, nil
		}}
}

// download is a route that sends a file.
func download(method, path, summary string, query []string, contentType string, write func(d *payload, r *http.Request, p *params) error) route {
	return downloadRoute(method, path, summary, query, contentType, nil, write)
}

// downloadWith sends a file for an optional JSON body.
func downloadWith[In any](method, path, summary, contentType string, write func(d *payload, r *http.Request, p *params, in In) error) route {
	rt := downloadRoute(method, path, summary, nil, contentType, typeOf[In](), func(d *payload, r *http.Request, p *params) error {
		var in In
		if err := decode(r, &in, true); err != nil {
			return err
		}

		return write(d, r, p, in)
	})
	rt.optionalBody = true

	return rt
}

// badRequest is a request the server can't read: a malformed body or
// parameter.
type badRequest struct{ message string }

func (e badRequest) Error() string { return e.message }

// decode reads a JSON body into v, refusing unknown fields (a typo would
// otherwise be ignored quietly).
func decode(r *http.Request, v any, optional bool) error {
	if kind := r.Header.Get("Content-Type"); kind != "" {
		if media, _, _ := mime.ParseMediaType(kind); media != "application/json" {
			return badRequest{"send the body as application/json"}
		}
	}

	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	err := decoder.Decode(v)
	if errors.Is(err, io.EOF) && optional {
		return nil
	}

	var tooLarge *http.MaxBytesError
	switch {
	case errors.Is(err, io.EOF):
		return badRequest{"a JSON body is required"}
	case errors.As(err, &tooLarge):
		return badRequest{"the body is too large"}
	case err != nil:
		return badRequest{"the body isn't valid: " + err.Error()}
	case decoder.More():
		return badRequest{"the body must be a single JSON value"}
	}

	return nil
}

// params are a request's path and query parameters, checked before the
// call: ids are whole numbers, lists are known, numbers and flags parse.
type params struct {
	r      *http.Request
	ids    map[string]uint
	values url.Values
	ints   map[string]int
	bools  map[string]bool
}

// idParams are path parameters that name a record.
var idParams = map[string]bool{"id": true, "logId": true, "paymentId": true}

func readParams(rt route, r *http.Request) (*params, error) {
	p := &params{r: r, ids: map[string]uint{}, values: r.URL.Query(), ints: map[string]int{}, bools: map[string]bool{}}
	for _, name := range pathParams(rt.path) {
		value := r.PathValue(name)
		switch {
		case idParams[name]:
			id, err := strconv.ParseUint(value, 10, 32)
			if err != nil || id == 0 {
				return nil, badRequest{fmt.Sprintf("%s must be a positive whole number", name)}
			}

			p.ids[name] = uint(id)
		case name == "list":
			if _, ok := settingLists[value]; !ok {
				return nil, notFound{fmt.Sprintf("there's no setting list %q", value)}
			}
		}
	}

	for _, spec := range rt.query {
		name, kind, _ := strings.Cut(spec, ":")
		raw := strings.TrimSpace(p.values.Get(name))
		if raw == "" {
			continue
		}

		switch kind {
		case "int":
			n, err := strconv.Atoi(raw)
			if err != nil {
				return nil, badRequest{fmt.Sprintf("%s must be a whole number", name)}
			}

			p.ints[name] = n
		case "bool":
			b, err := strconv.ParseBool(raw)
			if err != nil {
				return nil, badRequest{fmt.Sprintf("%s must be true or false", name)}
			}

			p.bools[name] = b
		}
	}

	return p, nil
}

// pathParams lists the {names} in path.
func pathParams(path string) []string {
	var names []string
	for _, part := range strings.Split(path, "/") {
		if strings.HasPrefix(part, "{") && strings.HasSuffix(part, "}") {
			names = append(names, strings.Trim(part, "{}"))
		}
	}

	return names
}

// currentMonth stands for no month in a path: the running one, where a call
// takes that (as the service does for an empty month).
const currentMonth = "current"

func (p *params) path(name string) string {
	value := p.r.PathValue(name)
	if name == "month" && value == currentMonth {
		return ""
	}

	return value
}

func (p *params) id(name string) uint { return p.ids[name] }

// list is the setting kind of the {list} in the path.
func (p *params) list() string { return settingLists[p.r.PathValue("list")] }

func (p *params) query(_ *http.Request, name string) string {
	return strings.TrimSpace(p.values.Get(name))
}

func (p *params) queryInt(_ *http.Request, name string, fallback int) int {
	if n, ok := p.ints[name]; ok {
		return n
	}

	return fallback
}

func (p *params) queryBool(_ *http.Request, name string, fallback bool) bool {
	if b, ok := p.bools[name]; ok {
		return b
	}

	return fallback
}

// queryInto fills the string fields of the struct v points to from the
// query parameters named by their JSON names.
func (p *params) queryInto(_ *http.Request, v any) {
	value := reflect.ValueOf(v).Elem()
	for i := 0; i < value.NumField(); i++ {
		if name := jsonName(value.Type().Field(i)); name != "" && value.Field(i).Kind() == reflect.String {
			value.Field(i).SetString(strings.TrimSpace(p.values.Get(name)))
		}
	}
}

// queryFields lists the query parameters queryInto reads into a v.
func queryFields(v any) []string {
	var names []string
	t := reflect.TypeOf(v)
	for i := 0; i < t.NumField(); i++ {
		if name := jsonName(t.Field(i)); name != "" && t.Field(i).Type.Kind() == reflect.String {
			names = append(names, name)
		}
	}

	return names
}

func jsonName(f reflect.StructField) string {
	name, _, _ := strings.Cut(f.Tag.Get("json"), ",")
	if name == "-" || !f.IsExported() {
		return ""
	}

	if name == "" {
		return f.Name
	}

	return name
}

// notFound is a path that names nothing.
type notFound struct{ message string }

func (e notFound) Error() string { return e.message }

// api serves the routes from a backend.
type api struct {
	log *slog.Logger
	// mu lets reads run together and writes one at a time: SQLite takes
	// one writer, and a change reads before it writes.
	mu sync.RWMutex
}

// mount adds the routes to mux, with a method-aware 404 and 405 for
// anything else under the API.
func (a *api) mount(mux *http.ServeMux, routes []route) {
	methods := map[string][]string{}
	for _, rt := range routes {
		pattern := apiPrefix + rt.path
		mux.Handle(rt.method+" "+pattern, a.handle(rt))
		methods[pattern] = append(methods[pattern], rt.method)
	}

	mux.HandleFunc(apiPrefix+"/", func(w http.ResponseWriter, r *http.Request) {
		// The mux matched no route; one with this path and another method
		// tells which methods it takes.
		for pattern, allowed := range methods {
			probe := r.Clone(r.Context())
			probe.Method = allowed[0]
			if _, matched := mux.Handler(probe); matched == allowed[0]+" "+pattern {
				w.Header().Set("Allow", strings.Join(allowed, ", "))
				writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "use "+strings.Join(allowed, " or "))
				return
			}
		}

		writeError(w, http.StatusNotFound, "not_found", "no such endpoint")
	})
}

func (a *api) handle(rt route) http.Handler {
	reads := rt.method == http.MethodGet
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p, err := readParams(rt, r)
		if err != nil {
			a.fail(w, r, err)
			return
		}

		if reads {
			a.mu.RLock()
		} else {
			a.mu.Lock()
		}

		result, err := rt.serve(w, r, p)

		if reads {
			a.mu.RUnlock()
		} else {
			a.mu.Unlock()
		}

		if err != nil {
			a.fail(w, r, err)
			return
		}

		if f, ok := result.(file); ok {
			for name, values := range f.headers {
				w.Header()[name] = values
			}

			w.Header().Set("Content-Length", strconv.Itoa(f.Len()))
			w.WriteHeader(rt.status)
			_, _ = f.WriteTo(w)
			return
		}

		if rt.out == nil {
			status := rt.status
			if status == http.StatusOK {
				status = http.StatusNoContent
			}

			w.WriteHeader(status)
			return
		}

		writeJSON(w, rt.status, result)
	})
}

// fail answers with err: the caller's mistakes say what to fix; the
// database's own failures are logged, and the client only learns that
// something went wrong.
func (a *api) fail(w http.ResponseWriter, r *http.Request, err error) {
	var bad badRequest
	var missing notFound
	switch {
	case errors.As(err, &bad):
		writeError(w, http.StatusBadRequest, "bad_request", bad.message)
		return
	case errors.As(err, &missing):
		writeError(w, http.StatusNotFound, "not_found", missing.message)
		return
	case errors.Is(err, service.ErrNoDatabase):
		writeError(w, http.StatusServiceUnavailable, "no_database", err.Error())
		return
	case errors.Is(err, service.ErrNotFound):
		writeError(w, http.StatusNotFound, "not_found", err.Error())
		return
	case errors.Is(err, service.ErrUpstream):
		writeError(w, http.StatusBadGateway, "rates_unavailable", err.Error())
		return
	}

	switch storage.Kind(err) {
	case storage.Missing:
		writeError(w, http.StatusNotFound, "not_found", err.Error())
	case storage.Conflict:
		var inUse storage.InUseError
		if errors.As(err, &inUse) {
			writeError(w, http.StatusConflict, "in_use", err.Error())
			return
		}

		a.log.Warn("conflict", "path", r.URL.Path, "client", info(r).client, "error", err)
		writeError(w, http.StatusConflict, "conflict", "that conflicts with a record there already")
	case storage.Failure:
		a.log.Error("storage failed", "path", r.URL.Path, "client", info(r).client, "error", err)
		writeError(w, http.StatusInternalServerError, "storage", "the database failed; see the server's log")
	default:
		writeError(w, http.StatusUnprocessableEntity, "invalid", err.Error())
	}
}

// Routes is the REST API (and /healthz) on b and files, without the
// guard: for a program that checks its callers its own way, like the bot's
// Mini App.
func Routes(b service.Backend, files service.Files, log *slog.Logger) http.Handler {
	return routes(b, files, log)
}

// routes is the whole server: health and the API.
func routes(b service.Backend, files service.Files, log *slog.Logger) *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok", "version": Version})
	})

	a := &api{log: log}
	all := apiRoutes(b, files)
	all = append(all, get("", "This description of the API (OpenAPI 3).", nil, func(_ *http.Request, _ *params) (map[string]any, error) {
		return describe(all), nil
	}).named("describe"))
	a.mount(mux, all)

	return mux
}
