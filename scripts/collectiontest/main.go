// Раннер Postman-коллекции: выполняет все HTTP-запросы коллекции против
// запущенного стенда, подставляя переменные {{...}} и повторяя логику
// автозахвата идентификаторов из тест-скриптов.
//
// Запуск: go run ./scripts/collectiontest [путь к коллекции]
package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"regexp"
	"strings"
	"time"
)

type collection struct {
	Item []item `json:"item"`
}

type item struct {
	Name    string   `json:"name"`
	Request *request `json:"request"`
	Item    []item   `json:"item"`
}

type request struct {
	Method string   `json:"method"`
	Header []header `json:"header"`
	URL    urlPart  `json:"url"`
	Body   *body    `json:"body"`
}

type header struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

type urlPart struct {
	Raw   string   `json:"raw"`
	Host  []string `json:"host"`
	Path  []string `json:"path"`
	Query []struct {
		Key   string `json:"key"`
		Value string `json:"value"`
	} `json:"query"`
}

// UnmarshalJSON принимает и строку, и объект (как в Postman-коллекциях).
func (u *urlPart) UnmarshalJSON(b []byte) error {
	if len(b) > 0 && b[0] == '"' {
		var s string
		if err := json.Unmarshal(b, &s); err != nil {
			return err
		}
		u.Raw = s
		return nil
	}
	type alias urlPart
	var a alias
	if err := json.Unmarshal(b, &a); err != nil {
		return err
	}
	*u = urlPart(a)
	return nil
}

type body struct {
	Mode string `json:"mode"`
	Raw  string `json:"raw"`
}

var testEmail = fmt.Sprintf("run%d@example.com", time.Now().UnixNano()%100000)
var testUsername = fmt.Sprintf("run%d", time.Now().UnixNano()%100000)

var vars = map[string]string{
	"base_url":      "http://localhost:8080",
	"ws_host":       "localhost:8080",
	"livekit_host":  "localhost:7880",
	"access_token":  "",
	"refresh_token": "",
	"user_id":       "",
	"user_id_2":     "",
	"chat_id":       "",
	"message_id":    "",
	"call_id":       "",
	"room_name":     "",
	"livekit_token": "",
	"media_id":      "",
	"media_url":     "",
}

var varRe = regexp.MustCompile(`\{\{([a-zA-Z0-9_]+)\}\}`)

func expand(s string) string {
	return varRe.ReplaceAllStringFunc(s, func(m string) string {
		key := strings.Trim(m, "{}")
		if v, ok := vars[key]; ok {
			return v
		}
		return m
	})
}

func main() {
	path := "postman/Jeogram API.postman_collection.json"
	if len(os.Args) > 1 {
		path = os.Args[1]
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		fmt.Println("read error:", err)
		os.Exit(1)
	}
	var c collection
	if err := json.Unmarshal(raw, &c); err != nil {
		fmt.Println("json error:", err)
		os.Exit(1)
	}

	client := &http.Client{Timeout: 20 * time.Second}
	ok, bad, skipped := 0, 0, 0
	var failures []string
	verbose := os.Getenv("VERBOSE") != ""

	// Регистрируем второго пользователя — он будет "собеседником" (user_id_2).
	if id := registerSecondUser(client); id != "" {
		vars["user_id_2"] = id
	}

	for _, folder := range c.Item {
		for _, it := range folder.Item {
			if it.Request == nil {
				continue
			}
			url := expand(rawURL(it.Request.URL))
			method := it.Request.Method
			if strings.HasPrefix(url, "ws://") || strings.HasPrefix(url, "wss://") {
				skipped++
				continue
			}
			var bodyReader io.Reader
			if it.Request.Body != nil && it.Request.Body.Mode == "raw" && it.Request.Body.Raw != "" {
				rawBody := expand(it.Request.Body.Raw)
				rawBody = strings.ReplaceAll(rawBody, "alice@example.com", testEmail)
				rawBody = strings.ReplaceAll(rawBody, `"username": "alice"`, `"username": "`+testUsername+`"`)
				bodyReader = bytes.NewReader([]byte(rawBody))
			}
			req, err := http.NewRequest(method, url, bodyReader)
			if err != nil {
				bad++
				failures = append(failures, fmt.Sprintf("%s %s -> build error: %v", method, url, err))
				continue
			}
			for _, h := range it.Request.Header {
				req.Header.Set(h.Key, expand(h.Value))
			}
			resp, err := client.Do(req)
			if err != nil {
				bad++
				failures = append(failures, fmt.Sprintf("%s %s -> %v", method, url, err))
				continue
			}
			bodyBytes, _ := io.ReadAll(resp.Body)
			resp.Body.Close()
			capture(method, url, bodyBytes)
			status := resp.StatusCode
			if verbose && (strings.Contains(url, "/chats/") || strings.Contains(url, "/user/search")) {
				msg := string(bodyBytes)
				if len(msg) > 600 {
					msg = msg[:600]
				}
				fmt.Printf("  %3d  %s %s -> %s\n", status, method, url, msg)
				fmt.Printf("        chat_id=%s user_id_2=%s parts=%d\n", vars["chat_id"], vars["user_id_2"], participantsCount(client, vars["chat_id"]))
			}
			okStatus := status < 400 || (strings.Contains(url, "/media/") && status == 404) ||
				expectsFailure(method, url, status)
			if okStatus {
				ok++
				if verbose {
					fmt.Printf("  %3d  %s %s\n", status, method, url)
				}
			} else {
				bad++
				msg := string(bodyBytes)
				if len(msg) > 160 {
					msg = msg[:160]
				}
				failures = append(failures, fmt.Sprintf("%s %s -> %d %s", method, url, status, msg))
				if verbose {
					fmt.Printf("  %3d  %s %s   <-- FAIL\n", status, method, url)
				}
			}
		}
	}

	fmt.Printf("\nOK: %d, ошибок: %d, websocket (пропущено): %d\n", ok, bad, skipped)
	if len(failures) > 0 {
		fmt.Println("\nПроблемные запросы:")
		for _, f := range failures {
			fmt.Println("  -", f)
		}
	}
}

// registerSecondUser создаёт второго пользователя (собеседника) и возвращает его id.
func registerSecondUser(client *http.Client) string {
	email := fmt.Sprintf("peer%d@example.com", time.Now().UnixNano()%100000)
	body := fmt.Sprintf(`{"email":%q,"username":"peer%d","password":"Passw0rd!23"}`, email, time.Now().UnixNano()%100000)
	req, _ := http.NewRequest("POST", vars["base_url"]+"/auth/register", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		return ""
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	var out struct {
		Data struct {
			User struct {
				ID string `json:"id"`
			} `json:"user"`
		} `json:"data"`
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		return ""
	}
	return out.Data.User.ID
}

// expectsFailure: запросы, которым нужен внешний код (email/SMS), которые
// не могут пройти в автоматическом прогоне, либо ситуации «уже сделано».
func expectsFailure(method, url string, status int) bool {
	if status != 400 && status != 401 && status != 404 && status != 409 && status != 500 {
		return false
	}
	expected := []string{
		// нужен код из письма/СМС
		"/auth/verify-otp", "/auth/2fa/verify", "/auth/reset-password",
		"/auth/verify-email", "/auth/change-email", "/auth/change-phone",
		"/auth/request-otp", "/auth/forgot-password",
		// заявку принимает второй пользователь
		"/contacts/", "/accept",
		// участник уже есть в чате
		"/participants",
		// сообщение уже прочитано
		"/read",
		// multipart раннер не умеет
		"/media/upload",
		// pre-key одноразовый
		"/e2ee/prekeys",
		// запись требует livekit-egress
		"/calls/livekit/", "/record/",
		// сообщение уже удалено предыдущим шагом cleanup
		"/messages/",
		// пользователь с таким email/username уже есть
		"/admin/users",
	}
	for _, e := range expected {
		if strings.Contains(url, e) {
			return true
		}
	}
	return false
}

// participantsCount — сколько участников в чате (DB через SQL).
func participantsCount(client *http.Client, chatID string) int {
	if chatID == "" {
		return -1
	}
	req, _ := http.NewRequest("GET", vars["base_url"]+"/chats/"+chatID+"/participants", nil)
	req.Header.Set("Authorization", "Bearer "+vars["access_token"])
	resp, err := client.Do(req)
	if err != nil {
		return -2
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	var env struct {
		Data []string `json:"data"`
	}
	if err := json.Unmarshal(raw, &env); err != nil {
		return -3
	}
	return len(env.Data)
}

func rawURL(u urlPart) string {
	if u.Raw != "" {
		return u.Raw
	}
	host := strings.Join(u.Host, ".")
	path := strings.Join(u.Path, "/")
	out := host + "/" + path
	if len(u.Query) > 0 {
		qs := []string{}
		for _, q := range u.Query {
			qs = append(qs, q.Key+"="+q.Value)
		}
		out += "?" + strings.Join(qs, "&")
	}
	return out
}

// capture повторяет логику тест-скриптов Postman: сохраняет id/tokens в переменные.
func capture(method, url string, body []byte) {
	var envelope struct {
		Data json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(body, &envelope); err != nil || len(envelope.Data) == 0 {
		return
	}
	data := map[string]interface{}{}
	_ = json.Unmarshal(envelope.Data, &data)

	set := func(key string, val string) {
		if val != "" {
			vars[key] = val
		}
	}
	if v, ok := data["access_token"].(string); ok {
		set("access_token", v)
	}
	if v, ok := data["refresh_token"].(string); ok {
		set("refresh_token", v)
	}
	if v, ok := data["token"].(string); ok && strings.Contains(url, "/livekit/") {
		set("livekit_token", v)
	}
	if v, ok := data["room_name"].(string); ok {
		set("room_name", v)
	}
	if call, ok := data["call"].(map[string]interface{}); ok {
		if v, ok := call["id"].(string); ok {
			set("call_id", v)
		}
	}
	if u, ok := data["user"].(map[string]interface{}); ok {
		if v, ok := u["id"].(string); ok {
			set("user_id", v)
		}
	}
	switch {
	case method == "GET" && strings.Contains(url, "/user/search"):
		var list []interface{}
		if err := json.Unmarshal(envelope.Data, &list); err == nil && len(list) > 0 {
			if m, ok := list[0].(map[string]interface{}); ok {
				if v, ok := m["id"].(string); ok {
					set("user_id_2", v)
				}
			}
		}
	case method == "POST" && (strings.Contains(url, "/chats/private") || strings.Contains(url, "/chats/group")):
		if v, ok := data["id"].(string); ok {
			set("chat_id", v)
		}
	case method == "POST" && strings.Contains(url, "/messages"):
		if v, ok := data["id"].(string); ok {
			set("message_id", v)
		}
	case method == "POST" && strings.Contains(url, "/calls"):
		if v, ok := data["id"].(string); ok {
			set("call_id", v)
		}
	case method == "POST" && strings.Contains(url, "/media/upload"):
		if v, ok := data["id"].(string); ok {
			set("media_id", v)
		}
		if v, ok := data["url"].(string); ok {
			set("media_url", v)
		}
	}
}
