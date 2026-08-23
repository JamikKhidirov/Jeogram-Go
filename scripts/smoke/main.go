package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/gorilla/websocket"
)

var base = "http://localhost:8080"

func post(path, token string, body map[string]interface{}) (map[string]interface{}, int) {
	b, _ := json.Marshal(body)
	req, _ := http.NewRequest("POST", base+path, bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, 0
	}
	defer resp.Body.Close()
	d, _ := io.ReadAll(resp.Body)
	var out map[string]interface{}
	_ = json.Unmarshal(d, &out)
	return out, resp.StatusCode
}

func get(path, token string) (map[string]interface{}, int) {
	req, _ := http.NewRequest("GET", base+path, nil)
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, 0
	}
	defer resp.Body.Close()
	d, _ := io.ReadAll(resp.Body)
	var out map[string]interface{}
	_ = json.Unmarshal(d, &out)
	return out, resp.StatusCode
}

func del(path, token string) (map[string]interface{}, int) {
	req, _ := http.NewRequest("DELETE", base+path, nil)
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, 0
	}
	defer resp.Body.Close()
	d, _ := io.ReadAll(resp.Body)
	var out map[string]interface{}
	_ = json.Unmarshal(d, &out)
	return out, resp.StatusCode
}

func accessOf(m map[string]interface{}) string {
	d, _ := m["data"].(map[string]interface{})
	if d == nil {
		return ""
	}
	if t, ok := d["access_token"].(string); ok {
		return t
	}
	if u, ok := d["user"].(map[string]interface{}); ok {
		if id, ok := u["id"].(string); ok {
			return id
		}
	}
	return ""
}

func idOf(m map[string]interface{}) string {
	d, _ := m["data"].(map[string]interface{})
	if d == nil {
		return ""
	}
	if id, ok := d["id"].(string); ok {
		return id
	}
	if u, ok := d["user"].(map[string]interface{}); ok {
		if id, ok := u["id"].(string); ok {
			return id
		}
	}
	return ""
}

func main() {
	pass := func(name string, ok bool, detail string) {
		s := "FAIL"
		if ok {
			s = "PASS"
		}
		fmt.Printf("[%s] %s %s\n", s, name, detail)
	}

	ru1 := fmt.Sprintf("wsA%d", time.Now().UnixNano())
	ru2 := fmt.Sprintf("wsB%d", time.Now().UnixNano())
	r1, _ := post("/auth/register", "", map[string]interface{}{"username": ru1, "email": ru1 + "@x.io", "password": "Passw0rd!23"})
	t1 := accessOf(r1)
	id1 := idOf(r1)
	r2, _ := post("/auth/register", "", map[string]interface{}{"username": ru2, "email": ru2 + "@x.io", "password": "Passw0rd!23"})
	t2 := accessOf(r2)
	id2 := idOf(r2)
	pass("register A", t1 != "" && id1 != "", fmt.Sprintf("uid=%s", id1))
	pass("register B", t2 != "" && id2 != "", fmt.Sprintf("uid=%s", id2))

	chat, _ := post("/chats/private", t1, map[string]interface{}{"user_id": id2})
	cid := idOf(chat)
	pass("create private chat", cid != "", fmt.Sprintf("chat=%s", cid))

	// Confirm token works on an authed HTTP route.
	prof, _ := get("/user/profile", t2)
	pass("token valid on /user/profile", prof != nil && prof["success"] == true, fmt.Sprintf("%v", prof))

	// Connect B over WebSocket.
	header := http.Header{}
	header.Set("Authorization", "Bearer "+t2)
	wsURL := "ws://localhost:8080/ws"
	conn, wsresp, err := websocket.DefaultDialer.Dial(wsURL, header)
	if err != nil {
		body := ""
		if wsresp != nil && wsresp.Body != nil {
			b, _ := io.ReadAll(wsresp.Body)
			body = string(b)
		}
		pass("ws connect (Bearer auth)", false, fmt.Sprintf("http=%d body=%s err=%s", wsresp.StatusCode, body, errMsg(err)))
		return
	}
	pass("ws connect (Bearer auth)", true, "")
	defer conn.Close()
	defer conn.Close()
	conn.SetReadDeadline(time.Now().Add(5 * time.Second))

	// A sends a message; B should receive message.new over WS.
	msg, _ := post("/messages", t1, map[string]interface{}{"chat_id": cid, "text": "hello ws", "type": "text"})
	mid := idOf(msg)
	pass("send message", mid != "", fmt.Sprintf("msg=%s", mid))

	got := false
	for i := 0; i < 3; i++ {
		_, raw, e := conn.ReadMessage()
		if e != nil {
			break
		}
		var env map[string]interface{}
		if json.Unmarshal(raw, &env) == nil {
			if env["type"] == "message.new" {
				got = true
				break
			}
		}
	}
	pass("ws received message.new", got, "")

	// Unread count for B should be >= 1, then drop to 0 after marking read.
	ub, _ := get("/chats/"+cid+"/unread", t2)
	ubn := intFrom(ub)
	pass("unread before > 0", ubn >= 1, fmt.Sprintf("unread=%d", ubn))

	_, _ = post("/messages/"+mid+"/read", t2, nil)
	ua, _ := get("/chats/"+cid+"/unread", t2)
	uan := intFrom(ua)
	pass("per-message read -> unread 0", uan == 0, fmt.Sprintf("unread=%d", uan))

	// Mute / unmute.
	mt, _ := post("/chats/"+cid+"/mute", t2, nil)
	mtStatus := statusOf(mt)
	um, _ := del("/chats/"+cid+"/mute", t2)
	umStatus := statusOf(um)
	pass("mute chat", mtStatus == "muted", fmt.Sprintf("status=%s", mtStatus))
	pass("unmute chat", umStatus == "unmuted", fmt.Sprintf("status=%s", umStatus))

	// Typing indicator (HTTP 200).
	_, tc := post("/chats/"+cid+"/typing", t1, nil)
	pass("typing indicator", tc == 200, fmt.Sprintf("http=%d", tc))
}

func intFrom(m map[string]interface{}) int {
	if m == nil {
		return -1
	}
	d, _ := m["data"].(map[string]interface{})
	if d == nil {
		return -1
	}
	if v, ok := d["unread"].(float64); ok {
		return int(v)
	}
	return -1
}

func statusOf(m map[string]interface{}) string {
	if m == nil {
		return ""
	}
	d, _ := m["data"].(map[string]interface{})
	if d == nil {
		return ""
	}
	if v, ok := d["status"].(string); ok {
		return v
	}
	return ""
}

func errMsg(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}
