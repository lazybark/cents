package client

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"

	"github.com/lazybark/cents/dbimport"
)

// ImportDatabase uploads a snapshot to merge, or previews it without saving.
func (c *Client) ImportDatabase(ctx context.Context, reader io.Reader, preview bool) (dbimport.Result, error) {
	target := *c.base
	target.Path += "/api/v1/import"
	query := target.Query()
	query.Set("preview", strconv.FormatBool(preview))
	target.RawQuery = query.Encode()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, target.String(), reader)
	if err != nil {
		return dbimport.Result{}, err
	}
	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Content-Type", "application/vnd.sqlite3")
	res, err := c.http.Do(req)
	if err != nil {
		return dbimport.Result{}, err
	}
	defer res.Body.Close()
	if res.StatusCode == http.StatusRequestEntityTooLarge {
		return dbimport.Result{}, dbimport.ErrTooLarge
	}
	if res.StatusCode >= 300 {
		var answer struct {
			Error struct {
				Code    string `json:"code"`
				Message string `json:"message"`
			} `json:"error"`
		}
		if err := json.NewDecoder(io.LimitReader(res.Body, 1<<20)).Decode(&answer); err != nil {
			return dbimport.Result{}, fmt.Errorf("the server answered %s", res.Status)
		}
		if answer.Error.Code == "too_large" {
			return dbimport.Result{}, dbimport.ErrTooLarge
		}
		if answer.Error.Message == "" {
			return dbimport.Result{}, errors.New("the server refused the database import")
		}
		return dbimport.Result{}, &Error{Status: res.StatusCode, Code: answer.Error.Code, Message: answer.Error.Message}
	}
	var result dbimport.Result
	err = json.NewDecoder(io.LimitReader(res.Body, 1<<20)).Decode(&result)
	return result, err
}
