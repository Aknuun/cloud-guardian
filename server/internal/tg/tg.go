// Package tg is a tiny Telegram Bot API client (polling, no deps).
package tg

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type Client struct {
	base string
	hc   *http.Client
}

func New(token string) *Client {
	return &Client{
		base: "https://api.telegram.org/bot" + token,
		hc:   &http.Client{Timeout: 60 * time.Second},
	}
}

type Chat struct {
	ID int64 `json:"id"`
}

// CopyText is the payload for a tap-to-copy button (Bot API copy_text).
type CopyText struct {
	Text string `json:"text"`
}

// Button is an inline keyboard button.
type Button struct {
	Text  string    `json:"text"`
	Data  string    `json:"callback_data,omitempty"`
	URL   string    `json:"url,omitempty"`
	Style string    `json:"style,omitempty"`
	Copy  *CopyText `json:"copy_text,omitempty"`
}

// Keyboard is an inline keyboard (rows of buttons).
type Keyboard [][]Button

type User struct {
	ID        int64  `json:"id"`
	Username  string `json:"username"`
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
}

type Message struct {
	MessageID int64  `json:"message_id"`
	Chat      Chat   `json:"chat"`
	From      *User  `json:"from"`
	Text      string `json:"text"`
}

type CallbackQuery struct {
	ID      string   `json:"id"`
	From    *User    `json:"from"`
	Message *Message `json:"message"`
	Data    string   `json:"data"`
}

type Update struct {
	UpdateID      int64          `json:"update_id"`
	Message       *Message       `json:"message"`
	CallbackQuery *CallbackQuery `json:"callback_query"`
}

type apiResp struct {
	OK          bool            `json:"ok"`
	Result      json.RawMessage `json:"result"`
	Description string          `json:"description"`
}

func (c *Client) call(method string, params map[string]any) (json.RawMessage, error) {
	form := url.Values{}
	for k, v := range params {
		var s string
		switch t := v.(type) {
		case string:
			s = t
		case int:
			s = fmt.Sprint(t)
		case int64:
			s = fmt.Sprint(t)
		default:
			b, _ := json.Marshal(t)
			s = string(b)
		}
		form.Set(k, s)
	}
	req, err := http.NewRequest("POST", c.base+"/"+method,
		bytes.NewBufferString(form.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := c.hc.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	var ar apiResp
	if err := json.Unmarshal(body, &ar); err != nil {
		return nil, err
	}
	if !ar.OK {
		d := ar.Description
		if !strings.Contains(d, "message is not modified") && !strings.Contains(d, "query is too old") {
			fmt.Println("tg error:", method, d)
		}
		return nil, fmt.Errorf("%s: %s", method, d)
	}
	return ar.Result, nil
}

func (c *Client) GetUpdates(offset int64, timeout int) ([]Update, error) {
	raw, err := c.call("getUpdates", map[string]any{
		"offset":          offset,
		"timeout":         timeout,
		"allowed_updates": `["message","callback_query"]`,
	})
	if err != nil {
		return nil, err
	}
	var out []Update
	return out, json.Unmarshal(raw, &out)
}

func wrapMarkup(kb any) any {
	if kb == nil {
		return nil
	}
	switch t := kb.(type) {
	case Keyboard:
		return map[string]any{"inline_keyboard": styleKB([][]Button(t))}
	case [][]Button:
		return map[string]any{"inline_keyboard": styleKB(t)}
	}
	return kb
}

// styleKB applies the same automatic button coloring as the server bot:
//   - data-list rows (domain/record/server...) keep their own look
//   - danger rows (delete/cancel/back/home...) turn red
//   - otherwise 1 & 3 button rows are green, 2 & 4+ are blue
var styleDataCB = []string{"cfz:", "cfr:", "hzs:", "hzsi:", "pgl:", "pgu:",
	"addrec:", "zf:", "cfp:", "sel:", "e:", "p:", "z:", "dd:", "sr:",
	"qanz:", "hzsv:", "hzn:"}

var styleDangerCB = []string{"del", "delete", "dely", "dy:", "dy", "stop",
	"cancel", "reset", "revoke"}

var styleDangerWords = []string{"حذف", "لغو", "انصراف", "غیرفعال", "توقف",
	"ریست", "بازنشانی", "پاک‌کردن", "بازگشت"}

func isPlaceholder(b Button) bool { return b.Text == " " }

func isBack(b Button) bool {
	t := b.Text
	if strings.Contains(t, "🏠") || strings.Contains(t, "🔙") ||
		strings.Contains(t, "⬅️") || strings.Contains(t, "بازگشت") ||
		strings.Contains(t, "منو") {
		return true
	}
	if b.Data == "menu" || b.Data == "back" || b.Data == "noop" {
		return strings.Contains(t, "🏠") || strings.Contains(t, "🔙") ||
			strings.Contains(t, "بازگشت") || strings.Contains(t, "منو")
	}
	return false
}

func rowIsDanger(row []Button) bool {
	for _, b := range row {
		if isPlaceholder(b) {
			continue
		}
		if b.Style == "danger" {
			return true
		}
		for _, k := range styleDangerCB {
			if strings.Contains(b.Data, k) {
				return true
			}
		}
		if strings.Contains(b.Text, "🗑") || strings.Contains(b.Text, "🛑") ||
			strings.Contains(b.Text, "🚫") {
			return true
		}
		for _, w := range styleDangerWords {
			if strings.Contains(b.Text, w) {
				return true
			}
		}
	}
	return false
}

func rowIsData(row []Button) bool {
	for _, b := range row {
		if isPlaceholder(b) {
			continue
		}
		for _, p := range styleDataCB {
			if strings.HasPrefix(b.Data, p) {
				return true
			}
		}
	}
	return false
}

func styleRow(row []Button) []Button {
	out := make([]Button, len(row))
	for i, b := range row {
		switch {
		case isPlaceholder(b):
			out[i] = b
		case b.Style == "plain":
			b.Style = ""
			out[i] = b
		case b.Style != "":
			out[i] = b
		case isBack(b):
			b.Style = ""
			out[i] = b
		case rowIsDanger([]Button{b}):
			b.Style = "danger"
			out[i] = b
		default:
			b.Style = ""
			out[i] = b
		}
	}
	return out
}

func styleKB(kb [][]Button) [][]Button {
	if kb == nil {
		return nil
	}
	out := make([][]Button, len(kb))
	for i, row := range kb {
		out[i] = styleRow(row)
	}
	return out
}

// Send posts a text message. kb is a Keyboard (or nil).
// Returns the message id (0 on error).
func (c *Client) Send(chat int64, text string, kb any) int64 {
	params := map[string]any{
		"chat_id":    chat,
		"text":       text,
		"parse_mode": "HTML",
	}
	if m := wrapMarkup(kb); m != nil {
		params["reply_markup"] = m
	}
	raw, err := c.call("sendMessage", params)
	if err != nil {
		return 0
	}
	var m Message
	_ = json.Unmarshal(raw, &m)
	return m.MessageID
}

func (c *Client) Edit(chat, msgID int64, text string, kb any) bool {
	params := map[string]any{
		"chat_id":    chat,
		"message_id": msgID,
		"text":       text,
		"parse_mode": "HTML",
	}
	if m := wrapMarkup(kb); m != nil {
		params["reply_markup"] = m
	}
	_, err := c.call("editMessageText", params)
	return err == nil
}

func (c *Client) Answer(cbID, text string) {
	_, _ = c.call("answerCallbackQuery", map[string]any{
		"callback_query_id": cbID, "text": text})
}

func (c *Client) SendDocument(chat int64, name string, content []byte, caption string) bool {
	// multipart upload
	return false
}

func (c *Client) DeleteWebhook() {
	_, _ = c.call("deleteWebhook", map[string]any{"drop_pending_updates": false})
}

// SetMyCommands registers the bot command menu shown in Telegram clients.
func (c *Client) SetMyCommands(cmds any) error {
	_, err := c.call("setMyCommands", map[string]any{"commands": cmds})
	return err
}

// SetChatMenuButton shows the command list in the chat menu button.
func (c *Client) SetChatMenuButton(commands bool) {
	mode := "default"
	if commands {
		mode = "commands"
	}
	_, _ = c.call("setChatMenuButton", map[string]any{
		"menu_button": map[string]any{"type": mode}})
}

func (c *Client) SetWebhook(u string) bool {
	_, err := c.call("setWebhook", map[string]any{
		"url": u, "drop_pending_updates": false})
	return err == nil
}

func (c *Client) GetWebhookInfo() (map[string]any, error) {
	raw, err := c.call("getWebhookInfo", map[string]any{})
	if err != nil {
		return nil, err
	}
	var m map[string]any
	return m, json.Unmarshal(raw, &m)
}

func (c *Client) GetMe() (map[string]any, error) {
	raw, err := c.call("getMe", map[string]any{})
	if err != nil {
		return nil, err
	}
	var m map[string]any
	return m, json.Unmarshal(raw, &m)
}
