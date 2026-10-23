// Package logview reads the diagnostic SQLite log database without modifying it.
package logview

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/url"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

type Query struct {
	Limit     int    `json:"limit"`
	BeforeID  int64  `json:"beforeId"`
	Level     string `json:"level"`
	Component string `json:"component"`
	Client    string `json:"client"`
	Status    string `json:"status"`
	Search    string `json:"search"`
	From      string `json:"from"`
	To        string `json:"to"`
}

type Entry struct {
	ID         int64  `json:"id"`
	Timestamp  string `json:"timestamp"`
	Level      string `json:"level"`
	Component  string `json:"component"`
	Message    string `json:"message"`
	Attributes string `json:"attributes"`
	Client     string `json:"client"`
	Status     *int   `json:"status"`
}

type Page struct {
	Entries    []Entry  `json:"entries"`
	Components []string `json:"components"`
	Clients    []string `json:"clients"`
	HasMore    bool     `json:"hasMore"`
	NextBefore int64    `json:"nextBefore"`
}

const TimeFormat = "2006-01-02T15:04:05.000000000Z"

// Project request fields out of attributes, so historical logs need no
// migration. Older gateway events used route for what is now called client.
const eventsSQL = `WITH metadata AS (
	SELECT *, CASE WHEN json_valid(attributes) THEN attributes ELSE '{}' END AS attrs FROM logs
), events AS (
	SELECT id, timestamp, level, component, message, attributes,
		CASE WHEN json_type(attrs, '$.status') = 'integer' AND json_extract(attrs, '$.status') BETWEEN 100 AND 599
			THEN json_extract(attrs, '$.status') END AS status,
		COALESCE(NULLIF(CASE WHEN json_type(attrs, '$.client') = 'text' THEN json_extract(attrs, '$.client') END, ''),
			CASE WHEN json_type(attrs, '$.route') = 'text' THEN json_extract(attrs, '$.route') END, '') AS client
	FROM metadata
) `

// Normalize bounds work per request and normalizes all time filters to UTC.
func (q Query) Normalize() (Query, error) {
	if q.Limit == 0 {
		q.Limit = 100
	}
	if q.Limit < 1 || q.Limit > 500 {
		return q, errors.New("limit must be between 1 and 500")
	}
	if q.BeforeID < 0 {
		return q, errors.New("beforeId must be positive")
	}
	q.Level = strings.ToUpper(strings.TrimSpace(q.Level))
	if q.Level != "" && q.Level != "DEBUG" && q.Level != "INFO" && q.Level != "WARN" && q.Level != "ERROR" {
		return q, errors.New("choose DEBUG, INFO, WARN or ERROR")
	}
	q.Component, q.Client, q.Search = strings.TrimSpace(q.Component), strings.TrimSpace(q.Client), strings.TrimSpace(q.Search)
	if len(q.Component) > 64 || len(q.Client) > 200 || len(q.Search) > 200 {
		return q, errors.New("component, client or search filter is too long")
	}
	q.Status = strings.ToLower(strings.TrimSpace(q.Status))
	if q.Status != "" {
		group := len(q.Status) == 3 && q.Status[0] >= '1' && q.Status[0] <= '5' && q.Status[1:] == "xx"
		code, err := strconv.Atoi(q.Status)
		if !group && (err != nil || len(q.Status) != 3 || code < 100 || code > 599) {
			return q, errors.New("HTTP status must be a code from 100 to 599 or a group such as 4xx")
		}
	}
	for _, field := range []*string{&q.From, &q.To} {
		if *field == "" {
			continue
		}
		stamp, err := time.Parse(time.RFC3339Nano, *field)
		if err != nil {
			return q, errors.New("time filters must be RFC3339 timestamps")
		}
		*field = stamp.UTC().Format(TimeFormat)
	}
	if q.From != "" && q.To != "" && q.To < q.From {
		return q, errors.New("the end time must not precede the start time")
	}
	return q, nil
}

// Read opens an existing file in read-only mode. A fresh connection observes
// committed WAL changes; an absent database is never silently created.
func Read(ctx context.Context, path string, query Query) (Page, error) {
	q, err := query.Normalize()
	if err != nil {
		return Page{}, err
	}
	if path == "" {
		return Page{}, errors.New("logs database is not configured")
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return Page{}, err
	}
	u := url.URL{Scheme: "file", Path: abs, RawQuery: "mode=ro&_query_only=1&_busy_timeout=1000"}
	db, err := sql.Open("sqlite3", u.String())
	if err != nil {
		return Page{}, err
	}
	defer db.Close()
	db.SetMaxOpenConns(1)
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	conditions, args := []string{"1=1"}, []any{}
	add := func(condition string, value any) {
		conditions = append(conditions, condition)
		args = append(args, value)
	}
	if q.BeforeID > 0 {
		add("id < ?", q.BeforeID)
	}
	if q.Level != "" {
		add("level = ?", q.Level)
	}
	if q.Component != "" {
		add("component = ?", q.Component)
	}
	if q.Client != "" {
		add("client = ?", q.Client)
	}
	if strings.HasSuffix(q.Status, "xx") {
		first := int(q.Status[0]-'0') * 100
		conditions = append(conditions, "status BETWEEN ? AND ?")
		args = append(args, first, first+99)
	} else if q.Status != "" {
		code, _ := strconv.Atoi(q.Status) // Normalize has validated it.
		add("status = ?", code)
	}
	if q.From != "" {
		add("timestamp >= ?", q.From)
	}
	if q.To != "" {
		add("timestamp <= ?", q.To)
	}
	if q.Search != "" {
		conditions = append(conditions, "(instr(lower(message), lower(?)) > 0 OR instr(lower(attributes), lower(?)) > 0)")
		args = append(args, q.Search, q.Search)
	}
	args = append(args, q.Limit+1)
	rows, err := db.QueryContext(ctx, eventsSQL+"SELECT id, timestamp, level, component, message, attributes, client, status FROM events WHERE "+strings.Join(conditions, " AND ")+" ORDER BY id DESC LIMIT ?", args...)
	if err != nil {
		return Page{}, fmt.Errorf("read logs: %w", err)
	}
	page := Page{Entries: make([]Entry, 0)}
	for rows.Next() {
		var e Entry
		if err := rows.Scan(&e.ID, &e.Timestamp, &e.Level, &e.Component, &e.Message, &e.Attributes, &e.Client, &e.Status); err != nil {
			_ = rows.Close()
			return Page{}, err
		}
		page.Entries = append(page.Entries, e)
	}
	err = rows.Err()
	_ = rows.Close()
	if err != nil {
		return Page{}, err
	}
	if len(page.Entries) > q.Limit {
		page.HasMore = true
		page.Entries = page.Entries[:q.Limit]
		page.NextBefore = page.Entries[len(page.Entries)-1].ID
	}
	page.Components, err = choices(ctx, db, "SELECT DISTINCT component FROM logs ORDER BY component LIMIT 100")
	if err != nil {
		return Page{}, err
	}
	page.Clients, err = choices(ctx, db, eventsSQL+"SELECT DISTINCT client FROM events WHERE client <> '' ORDER BY client LIMIT 100")
	return page, err
}

func choices(ctx context.Context, db *sql.DB, query string) ([]string, error) {
	rows, err := db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	values := make([]string, 0)
	for rows.Next() {
		var value string
		if err := rows.Scan(&value); err != nil {
			return nil, err
		}
		values = append(values, value)
	}
	return values, rows.Err()
}
