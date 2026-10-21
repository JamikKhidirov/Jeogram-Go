package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

const base = "http://localhost:8080"

func req(method, path, token string, body interface{}) (int, string) {
	var rdr io.Reader
	if body != nil {
		b, _ := json.Marshal(body)
		rdr = bytes.NewReader(b)
	}
	req, _ := http.NewRequest(method, base+path, rdr)
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return 0, "ERR " + err.Error()
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, string(b)
}

func main() {
	email := fmt.Sprintf("t%d@example.com", time.Now().UnixNano()%100000)
	code, body := req("POST", "/auth/register", "", map[string]interface{}{
		"email": email, "username": fmt.Sprintf("t%d", time.Now().UnixNano()%100000), "password": "Passw0rd!23"})
	fmt.Println("register:", code)
	var reg struct {
		Data struct {
			AccessToken string `json:"access_token"`
			User        struct {
				ID string `json:"id"`
			} `json:"user"`
		} `json:"data"`
	}
	_ = json.Unmarshal([]byte(body), &reg)
	tok := reg.Data.AccessToken

	// второй пользователь
	_, body2 := req("POST", "/auth/register", "", map[string]interface{}{
		"email": fmt.Sprintf("p%d@example.com", time.Now().UnixNano()%100000),
		"username": fmt.Sprintf("p%d", time.Now().UnixNano()%100000), "password": "Passw0rd!23"})
	var peer struct {
		Data struct {
			User struct {
				ID string `json:"id"`
			} `json:"user"`
		} `json:"data"`
	}
	_ = json.Unmarshal([]byte(body2), &peer)

	// чат
	code, body = req("POST", "/chats/private", tok, map[string]interface{}{"user_id": peer.Data.User.ID})
	fmt.Println("create chat:", code)
	var chat struct {
		Data struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	_ = json.Unmarshal([]byte(body), &chat)
	chatID := chat.Data.ID

	// POST /messages
	code, body = req("POST", "/messages", tok, map[string]interface{}{"chat_id": chatID, "type": "text", "text": "hi"})
	fmt.Println("POST /messages:", code)
	var msg struct {
		Data struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	_ = json.Unmarshal([]byte(body), &msg)

	// GET /messages/{id}
	code, body = req("GET", "/messages/"+msg.Data.ID, tok, nil)
	fmt.Println("GET /messages/{id}:", code, truncate(body))

	// POST /chats/{chat_id}/messages
	code, body = req("POST", "/chats/"+chatID+"/messages", tok, map[string]interface{}{"type": "text", "text": "hi2"})
	fmt.Println("POST /chats/{id}/messages:", code, truncate(body))

	// GET /media/{id} (несуществующий -> 404, валидный маршрут)
	code, body = req("GET", "/media/00000000-0000-0000-0000-000000000000", "", nil)
	fmt.Println("GET /media/{id}:", code, truncate(body))

	// POST /notifications/read с ids
	code, body = req("POST", "/notifications/read", tok, map[string]interface{}{"ids": []string{}})
	fmt.Println("POST /notifications/read {ids:[]}:", code, truncate(body))
	code, body = req("POST", "/notifications/read", tok, map[string]interface{}{"ids": []string{"x"}})
	fmt.Println("POST /notifications/read {ids:[x]}:", code, truncate(body))

	// GET /user/me и POST /user/avatar (были вне swagger)
	code, body = req("GET", "/user/me", tok, nil)
	fmt.Println("GET /user/me:", code, truncate(body))
	code, body = req("POST", "/user/avatar", tok, map[string]interface{}{"display_name": "Test Name"})
	fmt.Println("POST /user/avatar:", code, truncate(body))

	// GET /user/presence
	code, body = req("GET", "/user/presence?ids="+peer.Data.User.ID, tok, nil)
	fmt.Println("GET /user/presence:", code, truncate(body))

	// GET /calls/livekit/ice-servers
	code, body = req("GET", "/calls/livekit/ice-servers", tok, nil)
	fmt.Println("GET /calls/livekit/ice-servers:", code, truncate(body))

	_ = os.Stdout
}

func truncate(s string) string {
	if len(s) > 180 {
		return s[:180] + "..."
	}
	return strings.TrimSpace(s)
}
