package logview

import (
	"context"
	"database/sql"
	"net/url"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestReadFiltersAndStablePagination(t *testing.T) {
	path := filepath.Join(t.TempDir(), "logs ?#.db")
	u := url.URL{Scheme: "file", Path: path}
	db, err := sql.Open("sqlite3", u.String())
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	_, err = db.Exec(`CREATE TABLE logs(id INTEGER PRIMARY KEY, timestamp TEXT, level TEXT, component TEXT, message TEXT, attributes TEXT)`)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range []Entry{
		{ID: 1, Timestamp: "2026-10-23T10:00:00.000000000Z", Level: "INFO", Component: "server", Message: "started", Attributes: `{}`},
		{ID: 2, Timestamp: "2026-10-23T10:01:00.000000000Z", Level: "ERROR", Component: "bot", Message: "poll failed", Attributes: `{"user":123,"client":"mini_app_api","status":500}`},
		{ID: 3, Timestamp: "2026-10-23T10:02:00.000000000Z", Level: "ERROR", Component: "server", Message: "literal %_ ' OR 1=1 --", Attributes: `{"client":"desktop","status":404}`},
		{ID: 4, Timestamp: "2026-10-23T10:03:00.000000000Z", Level: "WARN", Component: "gateway", Message: "upstream failed", Attributes: `{"route":"mini_app_api","status":404}`},
	} {
		if _, err := db.Exec("INSERT INTO logs VALUES (?,?,?,?,?,?)", e.ID, e.Timestamp, e.Level, e.Component, e.Message, e.Attributes); err != nil {
			t.Fatal(err)
		}
	}
	read := func(q Query, want []int64) Page {
		t.Helper()
		page, err := Read(context.Background(), path, q)
		if err != nil {
			t.Fatal(err)
		}
		ids := []int64{}
		for _, e := range page.Entries {
			ids = append(ids, e.ID)
		}
		if !reflect.DeepEqual(ids, want) {
			t.Fatalf("query %+v: got %v, want %v", q, ids, want)
		}
		return page
	}
	first := read(Query{Limit: 2}, []int64{4, 3})
	if !first.HasMore || first.NextBefore != 3 || !reflect.DeepEqual(first.Components, []string{"bot", "gateway", "server"}) {
		t.Fatalf("unexpected page: %+v", first)
	}
	if _, err := db.Exec(`INSERT INTO logs VALUES (5,'2026-10-23T10:04:00.000000000Z','INFO','server','new','{}')`); err != nil {
		t.Fatal(err)
	}
	last := read(Query{Limit: 2, BeforeID: first.NextBefore}, []int64{2, 1})
	if last.HasMore || last.NextBefore != 0 {
		t.Fatalf("unexpected last page: %+v", last)
	}
	read(Query{Level: "error", Component: "server"}, []int64{3})
	read(Query{Search: "USER"}, []int64{2})
	read(Query{Search: "%_ ' OR 1=1 --"}, []int64{3})
	read(Query{Status: "404"}, []int64{4, 3})
	read(Query{Status: "4XX", Client: "mini_app_api"}, []int64{4})
	read(Query{Status: "5xx", Component: "bot"}, []int64{2})
	read(Query{Status: "200"}, []int64{})
	read(Query{Client: "' OR 1=1 --"}, []int64{})
	if !reflect.DeepEqual(first.Clients, []string{"desktop", "mini_app_api"}) || first.Entries[0].Status == nil || *first.Entries[0].Status != 404 || first.Entries[0].Client != "mini_app_api" {
		t.Fatalf("historical metadata was not exposed: %+v", first)
	}
	if last.Entries[1].Status != nil || last.Entries[1].Client != "" {
		t.Fatal("non-HTTP event was labeled as a request")
	}
	read(Query{Component: "' OR 1=1 --"}, []int64{})
	read(Query{From: "2026-10-23T13:01:00+03:00", To: "2026-10-23T10:02:00Z"}, []int64{3, 2})
	cancelled, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := Read(cancelled, path, Query{}); err == nil {
		t.Fatal("cancelled reads must stop")
	}
	if _, err := db.Exec(`INSERT INTO logs VALUES(6,'2026-10-23T10:05:00.000000000Z','INFO','cloud','malformed','broken JSON'),(7,'2026-10-23T10:06:00.000000000Z','INFO','cloud','wrong types','{"client":123,"status":"500"}')`); err != nil {
		t.Fatal(err)
	}
	malformed := read(Query{Component: "cloud"}, []int64{7, 6})
	for _, entry := range malformed.Entries {
		if entry.Client != "" || entry.Status != nil {
			t.Fatalf("malformed metadata became request data: %+v", entry)
		}
	}
}

func TestMissingDatabaseAndInvalidQueries(t *testing.T) {
	path := filepath.Join(t.TempDir(), "missing.db")
	if _, err := Read(context.Background(), path, Query{}); err == nil {
		t.Fatal("missing database should fail")
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("read created missing database: %v", err)
	}
	for _, q := range []Query{{Limit: -1}, {Limit: 501}, {BeforeID: -1}, {Level: "fatal"}, {From: "yesterday"}, {From: "2026-10-24T00:00:00Z", To: "2026-10-23T00:00:00Z"}, {Search: strings.Repeat("a", 201)}, {Client: strings.Repeat("a", 201)}, {Status: "600"}, {Status: "99"}, {Status: "404 OR 1=1"}, {Status: "6xx"}} {
		if _, err := q.Normalize(); err == nil {
			t.Fatalf("invalid query accepted: %+v", q)
		}
	}
}
