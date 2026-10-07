// Package telegram is the little of the Telegram Bot API cents-bot uses:
// long polling for updates, sending and editing messages with buttons, and
// answering button presses. See https://core.telegram.org/bots/api.
package telegram

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// DefaultAPI is Telegram's Bot API.
const DefaultAPI = "https://api.telegram.org"

// PollTimeout is how long a request for updates waits for one to come.
const PollTimeout = 50 * time.Second

// Client calls the Bot API as one bot.
type Client struct {
	api   string
	token string
	http  *http.Client
}

// New is a client of the bot with token (from @BotFather), calling the API
// at api (DefaultAPI, unless testing).
func New(api, token string) *Client {
	// Long polling holds a request open for PollTimeout.
	return &Client{api: strings.TrimRight(api, "/"), token: token, http: &http.Client{Timeout: PollTimeout + 15*time.Second}}
}

type User struct {
	ID        int64  `json:"id"`
	IsBot     bool   `json:"is_bot"`
	FirstName string `json:"first_name"`
	Username  string `json:"username"`
}

type Chat struct {
	ID   int64  `json:"id"`
	Type string `json:"type"`
}

type Message struct {
	MessageID int64  `json:"message_id"`
	From      *User  `json:"from"`
	Chat      Chat   `json:"chat"`
	Date      int64  `json:"date"`
	Text      string `json:"text"`
}

// CallbackQuery is a press of a button under a message.
type CallbackQuery struct {
	ID      string   `json:"id"`
	From    User     `json:"from"`
	Message *Message `json:"message"`
	Data    string   `json:"data"`
}

type Update struct {
	UpdateID      int64          `json:"update_id"`
	Message       *Message       `json:"message"`
	CallbackQuery *CallbackQuery `json:"callback_query"`
}

// Button is a button under a message; pressing it sends Data back (64
// bytes at most).
type Button struct {
	Text string `json:"text"`
	Data string `json:"callback_data"`
}

// Keyboard is rows of buttons.
type Keyboard [][]Button

// Command is one of the bot's commands, offered in Telegram's menu.
type Command struct {
	Command     string `json:"command"`
	Description string `json:"description"`
}

// Error is the API refusing a call.
type Error struct {
	Code        int
	Description string
	// RetryAfter is how long to wait, when calls come too fast.
	RetryAfter time.Duration
}

func (e *Error) Error() string {
	return fmt.Sprintf("telegram: %s (%d)", e.Description, e.Code)
}

// call posts params to method and reads its result into out.
func (c *Client) call(ctx context.Context, method string, params, out any) error {
	body, err := json.Marshal(params)
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.api+"/bot"+c.token+"/"+method, bytes.NewReader(body))
	if err != nil {
		return errors.New("telegram: the API address isn't valid")
	}

	req.Header.Set("Content-Type", "application/json")
	res, err := c.http.Do(req)
	if err != nil {
		// The request's address holds the token: only the cause is kept.
		var failed *url.Error
		if errors.As(err, &failed) {
			err = failed.Err
		}

		return fmt.Errorf("telegram: %s: %w", method, err)
	}
	defer res.Body.Close()

	var answer struct {
		OK          bool            `json:"ok"`
		Result      json.RawMessage `json:"result"`
		ErrorCode   int             `json:"error_code"`
		Description string          `json:"description"`
		Parameters  struct {
			RetryAfter int `json:"retry_after"`
		} `json:"parameters"`
	}

	if err := json.NewDecoder(res.Body).Decode(&answer); err != nil {
		return fmt.Errorf("telegram: %s: the answer isn't valid (%s)", method, res.Status)
	}

	if !answer.OK {
		return &Error{Code: answer.ErrorCode, Description: answer.Description, RetryAfter: time.Duration(answer.Parameters.RetryAfter) * time.Second}
	}

	if out == nil {
		return nil
	}

	return json.Unmarshal(answer.Result, out)
}

// Me is the bot itself; it checks the token.
func (c *Client) Me(ctx context.Context) (User, error) {
	var me User
	err := c.call(ctx, "getMe", struct{}{}, &me)
	return me, err
}

// Updates waits up to PollTimeout for updates after offset (the last one
// seen, plus one), which also confirms those before it.
func (c *Client) Updates(ctx context.Context, offset int64) ([]Update, error) {
	var updates []Update
	err := c.call(ctx, "getUpdates", map[string]any{
		"offset":          offset,
		"timeout":         int(PollTimeout / time.Second),
		"allowed_updates": []string{"message", "callback_query"},
	}, &updates)

	return updates, err
}

// Send sends text (HTML) to chat, with buttons if keyboard isn't nil; it
// returns the message's ID.
func (c *Client) Send(ctx context.Context, chat int64, text string, keyboard Keyboard) (int64, error) {
	params := map[string]any{"chat_id": chat, "text": text, "parse_mode": "HTML", "link_preview_options": map[string]bool{"is_disabled": true}}
	if keyboard != nil {
		params["reply_markup"] = map[string]any{"inline_keyboard": keyboard}
	}

	var sent Message
	err := c.call(ctx, "sendMessage", params, &sent)
	return sent.MessageID, err
}

// Edit replaces a message's text (HTML) and buttons; nil keyboard removes
// them.
func (c *Client) Edit(ctx context.Context, chat, message int64, text string, keyboard Keyboard) error {
	if keyboard == nil {
		keyboard = Keyboard{}
	}

	err := c.call(ctx, "editMessageText", map[string]any{
		"chat_id": chat, "message_id": message, "text": text, "parse_mode": "HTML",
		"link_preview_options": map[string]bool{"is_disabled": true},
		"reply_markup":         map[string]any{"inline_keyboard": keyboard},
	}, nil)

	// Pressing a button twice edits the message to what it says already.
	var refused *Error
	if errors.As(err, &refused) && strings.Contains(refused.Description, "message is not modified") {
		return nil
	}

	return err
}

// Answer tells Telegram a button press was handled; text (if any) shows
// briefly to the user.
func (c *Client) Answer(ctx context.Context, callback, text string) error {
	return c.call(ctx, "answerCallbackQuery", map[string]any{"callback_query_id": callback, "text": text}, nil)
}

// SetCommands sets the commands offered in Telegram's menu.
func (c *Client) SetCommands(ctx context.Context, commands []Command) error {
	return c.call(ctx, "setMyCommands", map[string]any{"commands": commands}, nil)
}

// MenuButton is the button next to a chat's message field: "commands"
// (the list of commands), "web_app" (opening URL) or "default".
type MenuButton struct {
	Type   string `json:"type"`
	Text   string `json:"text"`
	WebApp struct {
		URL string `json:"url"`
	} `json:"web_app"`
}

// SetMenuButton makes the menu button open the Mini App at url, labelled
// text: in the private chat with chat, or (chat 0) in every private chat
// that has no button of its own.
func (c *Client) SetMenuButton(ctx context.Context, chat int64, text, url string) error {
	params := map[string]any{"menu_button": map[string]any{"type": "web_app", "text": text, "web_app": map[string]string{"url": url}}}
	if chat != 0 {
		params["chat_id"] = chat
	}

	return c.call(ctx, "setChatMenuButton", params, nil)
}

// GetMenuButton is the menu button of the private chat with chat, or
// (chat 0) every private chat's.
func (c *Client) GetMenuButton(ctx context.Context, chat int64) (MenuButton, error) {
	params := map[string]any{}
	if chat != 0 {
		params["chat_id"] = chat
	}

	var button MenuButton
	err := c.call(ctx, "getChatMenuButton", params, &button)
	return button, err
}

// Escape makes text safe inside an HTML message.
func Escape(text string) string {
	return strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;").Replace(text)
}
