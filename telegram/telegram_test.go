package telegram

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

const token = "123:secret-token"

// fakeAPI answers like the Bot API, keeping what each method was sent.
func fakeAPI(t *testing.T, answers map[string]string) (*httptest.Server, map[string]map[string]any) {
	t.Helper()

	got := map[string]map[string]any{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		method, ok := strings.CutPrefix(r.URL.Path, "/bot"+token+"/")
		if !ok {
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(`{"ok":false,"error_code":404,"description":"Not Found"}`))
			return
		}

		var params map[string]any
		_ = json.NewDecoder(r.Body).Decode(&params)
		got[method] = params

		answer, ok := answers[method]
		if !ok {
			answer = `{"ok":true,"result":true}`
		}

		_, _ = w.Write([]byte(answer))
	}))
	t.Cleanup(srv.Close)

	return srv, got
}

func TestClient(t *testing.T) {
	ctx := context.Background()
	srv, got := fakeAPI(t, map[string]string{
		"getMe":               `{"ok":true,"result":{"id":42,"is_bot":true,"first_name":"cents","username":"cents_bot"}}`,
		"sendMessage":         `{"ok":true,"result":{"message_id":77,"chat":{"id":5}}}`,
		"getUpdates":          `{"ok":true,"result":[{"update_id":3,"message":{"message_id":1,"from":{"id":5},"chat":{"id":5},"text":"hi"}},{"update_id":4,"callback_query":{"id":"cb","from":{"id":5},"data":"u:1"}}]}`,
		"editMessageText":     `{"ok":false,"error_code":400,"description":"Bad Request: message is not modified"}`,
		"answerCallbackQuery": `{"ok":false,"error_code":429,"description":"Too Many Requests","parameters":{"retry_after":3}}`,
		"getChatMenuButton":   `{"ok":true,"result":{"type":"web_app","text":"cents","web_app":{"url":"https://cents.example.com/"}}}`,
	})
	c := New(srv.URL+"/", token)

	if me, err := c.Me(ctx); err != nil || me.ID != 42 || me.Username != "cents_bot" {
		t.Fatalf("unexpected me %+v %v", me, err)
	}

	id, err := c.Send(ctx, 5, "<b>hi</b>", Keyboard{{{Text: "Undo", Data: "u:1"}}})
	sent := got["sendMessage"]
	keyboard := sent["reply_markup"].(map[string]any)["inline_keyboard"].([]any)[0].([]any)[0].(map[string]any)
	if err != nil || id != 77 || sent["parse_mode"] != "HTML" || sent["chat_id"] != 5.0 || keyboard["callback_data"] != "u:1" {
		t.Fatalf("unexpected send %d %v %v", id, err, sent)
	}

	if _, err := c.Send(ctx, 5, "plain", nil); err != nil || got["sendMessage"]["reply_markup"] != nil {
		t.Fatalf("no keyboard sends none: %v %v", err, got["sendMessage"])
	}

	updates, err := c.Updates(ctx, 3)
	if err != nil || len(updates) != 2 || updates[0].Message.Text != "hi" || updates[1].CallbackQuery.Data != "u:1" || got["getUpdates"]["offset"] != 3.0 || got["getUpdates"]["timeout"] != 50.0 {
		t.Fatalf("unexpected updates %+v %v %v", updates, err, got["getUpdates"])
	}

	// Editing to the same text again is fine.
	if err := c.Edit(ctx, 5, 77, "same", nil); err != nil {
		t.Fatalf("expected an unchanged edit ignored: %v", err)
	}

	err = c.Answer(ctx, "cb", "")
	var refused *Error
	if !errors.As(err, &refused) || refused.Code != 429 || refused.RetryAfter != 3*time.Second {
		t.Fatalf("expected a retry asked for: %v", err)
	}

	if err := c.SetCommands(ctx, []Command{{Command: "month", Description: "This month"}}); err != nil || got["setMyCommands"]["commands"] == nil {
		t.Fatalf("unexpected commands %v %v", err, got["setMyCommands"])
	}

	if err := c.SetMenuButton(ctx, 0, "Open", "https://cents.example.com/"); err != nil || got["setChatMenuButton"]["chat_id"] != nil {
		t.Fatalf("expected every chat's button: %v %v", err, got["setChatMenuButton"])
	}

	button := got["setChatMenuButton"]["menu_button"].(map[string]any)
	if button["type"] != "web_app" || button["web_app"].(map[string]any)["url"] != "https://cents.example.com/" {
		t.Fatalf("unexpected menu button %v", button)
	}

	if err := c.SetMenuButton(ctx, 5, "Open", "https://cents.example.com/"); err != nil || got["setChatMenuButton"]["chat_id"] != 5.0 {
		t.Fatalf("expected one chat's button: %v %v", err, got["setChatMenuButton"])
	}

	if menu, err := c.GetMenuButton(ctx, 5); err != nil || menu.Type != "web_app" || menu.WebApp.URL != "https://cents.example.com/" || got["getChatMenuButton"]["chat_id"] != 5.0 {
		t.Fatalf("unexpected menu button %+v %v", menu, err)
	}

	// A wrong token: the API's answer, and never the token itself.
	wrong := New(srv.URL, "nope")
	if _, err := wrong.Me(ctx); !errors.As(err, &refused) || refused.Code != 404 {
		t.Fatalf("expected the token refused: %v", err)
	}

	gone := New("http://127.0.0.1:1", token)
	if _, err := gone.Me(ctx); err == nil || strings.Contains(err.Error(), "secret-token") {
		t.Fatalf("expected a failure without the token in it: %v", err)
	}
}

func TestEscape(t *testing.T) {
	if got := Escape(`<b>Tom & Jerry</b>`); got != "&lt;b&gt;Tom &amp; Jerry&lt;/b&gt;" {
		t.Fatalf("unexpected %q", got)
	}
}
