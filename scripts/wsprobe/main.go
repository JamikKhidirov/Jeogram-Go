// Проверка всех realtime-событий Jeogram: /ws и /socket.io.
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

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
)

const base = "http://localhost:8080"

func post(path string, body interface{}, token string) map[string]interface{} {
	b, _ := json.Marshal(body)
	req, _ := http.NewRequest("POST", base+path, bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		fmt.Println("POST", path, "ERR", err)
		return map[string]interface{}{}
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	var out map[string]interface{}
	_ = json.Unmarshal(raw, &out)
	return out
}

func get(path, token string) map[string]interface{} {
	req, _ := http.NewRequest("GET", base+path, nil)
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return map[string]interface{}{}
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	var out map[string]interface{}
	_ = json.Unmarshal(raw, &out)
	return out
}

func register() (string, string) {
	email := fmt.Sprintf("rt%d@example.com", time.Now().UnixNano()%100000)
	r := post("/auth/register", map[string]interface{}{
		"email": email, "username": "rt" + uuid.NewString()[:8], "password": "Passw0rd!23"}, "")
	d, _ := r["data"].(map[string]interface{})
	tok, _ := d["access_token"].(string)
	u, _ := d["user"].(map[string]interface{})
	id, _ := u["id"].(string)
	return tok, id
}

// waitEvent читает события, пока не найдёт нужный тип (или не выйдет время).
func waitEvent(c *websocket.Conn, want string, timeout time.Duration) string {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		c.SetReadDeadline(deadline)
		_, msg, err := c.ReadMessage()
		if err != nil {
			return ""
		}
		s := string(msg)
		if strings.Contains(s, want) {
			return s
		}
	}
	return ""
}

func main() {
	tokA, idA := register()
	tokB, idB := register()
	fmt.Println("users:", idA, idB)

	// Контакт: B -> A (B отправил заявку), A принимает её.
	// ВАЖНО: POST /contacts/{user_id}/accept вызывает ПОЛУЧАТЕЛЬ, а {user_id} —
	// это id инициатора заявки (того, кто её отправил).
	post("/contacts", map[string]interface{}{"user_id": idA}, tokB)
	acc := post("/contacts/"+idB+"/accept", map[string]interface{}{}, tokA)
	if acc["success"] != true {
		fmt.Println("accept failed:", acc)
	}

	chat := post("/chats/private", map[string]interface{}{"user_id": idB}, tokA)
	chatID, _ := chat["data"].(map[string]interface{})["id"].(string)
	fmt.Println("chat:", chatID)

	cA, _, err := websocket.DefaultDialer.Dial("ws://localhost:8080/ws?token="+tokA, nil)
	if err != nil {
		fmt.Println("dial A:", err)
		os.Exit(1)
	}
	defer cA.Close()

	results := map[string]bool{}

	// 1) message.new
	msg := post("/messages", map[string]interface{}{"chat_id": chatID, "type": "text", "text": "hello"}, tokA)
	msgID, _ := msg["data"].(map[string]interface{})["id"].(string)
	ev := waitEvent(cA, "message.new", 5*time.Second)
	results["message.new"] = ev != ""
	fmt.Println("message.new:", first(ev))

	// 2) message.read (B отмечает прочитанным -> событие A)
	post("/chats/"+chatID+"/read", map[string]interface{}{"message_ids": []string{msgID}}, tokB)
	ev = waitEvent(cA, "message.read", 5*time.Second)
	results["message.read"] = ev != ""
	fmt.Println("message.read:", first(ev))

	// 3) notification (in-app уведомление для B о сообщении A)
	notif := get("/notifications", tokB)
	arr, _ := notif["data"].([]interface{})
	results["notification"] = len(arr) > 0
	fmt.Println("notification (в БД):", len(arr))

	// 4) profile.updated (A меняет профиль)
	put("/user/profile", map[string]interface{}{"display_name": "New Name"}, tokA)
	ev = waitEvent(cA, "profile.updated", 5*time.Second)
	results["profile.updated"] = ev != ""
	fmt.Println("profile.updated:", first(ev))

	// 5) user.status
	post("/user/status", map[string]interface{}{"status": "dnd"}, tokA)
	ev = waitEvent(cA, "user.status", 5*time.Second)
	results["user.status"] = ev != ""
	fmt.Println("user.status:", first(ev))

	// 6) call.started + call.ended
	call := post("/calls", map[string]interface{}{"chat_id": chatID, "type": "audio"}, tokA)
	callID, _ := call["data"].(map[string]interface{})["id"].(string)
	ev = waitEvent(cA, "call.started", 5*time.Second)
	results["call.started"] = ev != ""
	fmt.Println("call.started:", first(ev))
	post("/calls/"+callID+"/end", map[string]interface{}{}, tokA)
	ev = waitEvent(cA, "call.ended", 5*time.Second)
	results["call.ended"] = ev != ""
	fmt.Println("call.ended:", first(ev))

	// 7) presence (A подключается -> B (контакт A) получает presence)
	cB, _, err := websocket.DefaultDialer.Dial("ws://localhost:8080/ws?token="+tokB, nil)
	if err == nil {
		defer cB.Close()
		// B отключается и снова подключается, чтобы A увидел presence
		cB.Close()
		cB2, _, err2 := websocket.DefaultDialer.Dial("ws://localhost:8080/ws?token="+tokB, nil)
		if err2 == nil {
			defer cB2.Close()
			ev = waitEvent(cA, "presence", 5*time.Second)
			results["presence"] = ev != ""
			fmt.Println("presence:", first(ev))
		}
	}

	// 8) Socket.IO: message.new
	cS, _, err := websocket.DefaultDialer.Dial("ws://localhost:8080/socket.io/?token="+tokA+"&EIO=3&transport=websocket", nil)
	if err == nil {
		defer cS.Close()
		_ = cS.WriteMessage(websocket.TextMessage, []byte("2"))
		waitEvent(cS, "3", 3*time.Second)
		_ = cS.WriteMessage(websocket.TextMessage, []byte("40"))
		post("/messages", map[string]interface{}{"chat_id": chatID, "type": "text", "text": "sio"}, tokA)
		ev = waitEvent(cS, "42[", 5*time.Second)
		results["socket.io message.new"] = ev != ""
		fmt.Println("socket.io event:", first(ev))
	} else {
		results["socket.io message.new"] = false
		fmt.Println("socket.io dial:", err)
	}

	fmt.Println("\nИТОГ:")
	allOK := true
	for _, k := range []string{"message.new", "message.read", "notification", "profile.updated",
		"user.status", "call.started", "call.ended", "presence", "socket.io message.new"} {
		mark := "OK  "
		if !results[k] {
			mark = "FAIL"
			allOK = false
		}
		fmt.Printf("  %s %s\n", mark, k)
	}
	if !allOK {
		os.Exit(1)
	}
}

func put(path string, body interface{}, token string) {
	b, _ := json.Marshal(body)
	req, _ := http.NewRequest("PUT", base+path, bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := http.DefaultClient.Do(req)
	if err == nil {
		resp.Body.Close()
	}
}

func first(s string) string {
	if s == "" {
		return "<не получено>"
	}
	if len(s) > 170 {
		return s[:170] + "..."
	}
	return s
}
