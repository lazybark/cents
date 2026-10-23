package client

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"

	"github.com/lazybark/cents/logview"
)

// ReadLogs reads the separate administration API using an admin-capable token.
func (c *Client) ReadLogs(ctx context.Context, query logview.Query) (logview.Page, error) {
	q, err := query.Normalize()
	if err != nil {
		return logview.Page{}, err
	}
	target := *c.base
	target.Path += "/api/v1/admin/logs"
	values := url.Values{"limit": {strconv.Itoa(q.Limit)}}
	if q.BeforeID > 0 {
		values.Set("beforeId", strconv.FormatInt(q.BeforeID, 10))
	}
	for key, value := range map[string]string{"level": q.Level, "component": q.Component, "client": q.Client, "status": q.Status, "search": q.Search, "from": q.From, "to": q.To} {
		if value != "" {
			values.Set(key, value)
		}
	}
	target.RawQuery = values.Encode()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, target.String(), nil)
	if err != nil {
		return logview.Page{}, err
	}
	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Accept", "application/json")
	res, err := c.http.Do(req)
	if err != nil {
		return logview.Page{}, err
	}
	defer res.Body.Close()
	if res.StatusCode >= 300 {
		var answer struct {
			Error struct {
				Code    string `json:"code"`
				Message string `json:"message"`
			} `json:"error"`
		}
		if err := json.NewDecoder(io.LimitReader(res.Body, 1<<20)).Decode(&answer); err != nil || answer.Error.Message == "" {
			return logview.Page{}, fmt.Errorf("the server answered %s", res.Status)
		}
		return logview.Page{}, &Error{Status: res.StatusCode, Code: answer.Error.Code, Message: answer.Error.Message}
	}
	var page logview.Page
	err = json.NewDecoder(res.Body).Decode(&page)
	return page, err
}
