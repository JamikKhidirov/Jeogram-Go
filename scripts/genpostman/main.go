package main

import (
	"encoding/json"
	"os"
	"strings"
)

type Header struct {
	Key   string `json:"key"`
	Value string `json:"value"`
	Type  string `json:"type"`
}

type URL struct {
	Raw  string            `json:"raw"`
	Host []string          `json:"host"`
	Port string            `json:"port,omitempty"`
	Path []string          `json:"path"`
	Query []queryParam     `json:"query,omitempty"`
}

type queryParam struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

type Body struct {
	Mode       string `json:"mode"`
	Raw        string `json:"raw"`
	Options    map[string]map[string]string `json:"options,omitempty"`
}

type Request struct {
	Method      string   `json:"method"`
	Header      []Header `json:"header"`
	URL         URL      `json:"url"`
	Body        *Body    `json:"body,omitempty"`
	Description string   `json:"description,omitempty"`
}

type Item struct {
	Name        string   `json:"name"`
	Event       []event  `json:"event,omitempty"`
	Request     Request  `json:"request"`
	Description string   `json:"description,omitempty"`
}

type Folder struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	Item        []Item `json:"item"`
}

type Collection struct {
	Info struct {
		Name        string `json:"name"`
		Description string `json:"description"`
		Schema      string `json:"schema"`
	} `json:"info"`
	Item     []interface{} `json:"item"`
	Variable []variable    `json:"variable"`
	Event    []event       `json:"event,omitempty"`
}

type variable struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

type event struct {
	Listen string `json:"listen"`
	Script struct {
		Type string   `json:"type"`
		Exec []string `json:"exec"`
	} `json:"script"`
}

func makeURL(path string, query []queryParam) URL {
	parts := strings.Split(strings.Trim(path, "/"), "/")
	u := URL{Raw: "{{baseUrl}}/" + strings.Trim(path, "/"), Host: []string{"{{baseUrl}}"}, Port: "", Path: append([]string{""}, parts...)}
	u.Query = query
	return u
}

func req(method, path, body, desc string, protected bool, query []queryParam) Request {
	r := Request{Method: method, URL: makeURL(path, query), Description: desc}
	if protected {
		r.Header = append(r.Header, Header{Key: "Authorization", Value: "Bearer {{accessToken}}", Type: "text"})
	}
	r.Header = append(r.Header, Header{Key: "Content-Type", Value: "application/json", Type: "text"})
	if body != "" {
		r.Body = &Body{Mode: "raw", Raw: body, Options: map[string]map[string]string{"raw": {"language": "json"}}}
	}
	return r
}

func item(name, method, path, body, desc string, protected bool, query []queryParam) Item {
	return Item{Name: name, Request: req(method, path, body, desc, protected, query), Description: desc}
}

func main() {
	c := Collection{}
	c.Info.Name = "Jeogram Messenger API"
	c.Info.Description = "Коллекция Jeogram Messenger. 1) Откройте папку Auth → Register, затем Login (токен сохраняется автоматически). 2) Остальные запросы используют Bearer {{accessToken}}. 3) Для realtime см. описание папки Realtime (WebSocket) и файл SOCKETIO.md."
	c.Info.Schema = "https://schema.getpostman.com/json/collection/v2.1.0/collection.json"
	c.Variable = []variable{
		{Key: "baseUrl", Value: "http://localhost:8080"},
		{Key: "accessToken", Value: ""},
		{Key: "chatId", Value: ""},
		{Key: "userId", Value: ""},
		{Key: "messageId", Value: ""},
	}

	// ----- Auth -----
	auth := Folder{Name: "Auth", Description: "Регистрация и получение JWT. Login сохраняет токен в переменную accessToken."}
	auth.Item = append(auth.Item,
		item("Register", "POST", "/auth/register", `{"username":"alice","email":"alice@example.com","password":"Passw0rd!23"}`, "Создать пользователя. Скопируйте id для других запросов.", false, nil),
		item("Login", "POST", "/auth/login", `{"username":"alice","password":"Passw0rd!23"}`, "Вход. Скрипт теста сохраняет data.access_token в {{accessToken}}.", false, nil),
		item("Refresh", "POST", "/auth/refresh", `{"refresh_token":"<refresh>"}`, "Обновить access-токен.", false, nil),
		item("Me", "GET", "/auth/me", "", "Кто я (проверка токена).", true, nil),
		item("Logout", "POST", "/auth/logout", "", "Выход (инвалидация сессии).", true, nil),
	)
	// login token capture
	loginItem := &auth.Item[1]
	loginItem.Event = []event{{
		Listen: "test",
		Script: struct {
			Type string   `json:"type"`
			Exec []string `json:"exec"`
		}{
			Type: "text/javascript",
			Exec: []string{
				"pm.test(\"login ok\", function () { pm.response.to.have.status(200); });",
				"var json = pm.response.json();",
				"if (json.data && json.data.access_token) {",
				"    pm.collectionVariables.set(\"accessToken\", json.data.access_token);",
				"    console.log(\"accessToken saved\");",
				"}",
			},
		},
	}}

	// ----- User -----
	user := Folder{Name: "User", Description: "Профиль, настройки, поиск, присутствие, блокировки, экспорт, удаление аккаунта."}
	user.Item = append(user.Item,
		item("Get profile", "GET", "/user/profile", "", "Профиль текущего пользователя.", true, nil),
		item("Update profile", "PUT", "/user/profile", `{"display_name":"Alice","avatar_url":"https://...","bio":"hi"}`, "Обновить профиль.", true, nil),
		item("Get settings", "GET", "/user/settings", "", "Настройки.", true, nil),
		item("Update settings", "PUT", "/user/settings", `{"theme":"dark","notifications_enabled":true,"language":"ru"}`, "Настройки.", true, nil),
		item("Search users", "GET", "/user/search", "", "Поиск пользователей.", true, []queryParam{{Key: "q", Value: "alice"}}),
		item("Presence", "GET", "/user/presence", "", "Онлайн-статусы (через ids).", true, []queryParam{{Key: "ids", Value: "{{userId}}"}}),
		item("Export data", "GET", "/user/export", "", "GDPR-выгрузка данных.", true, nil),
		item("Block user", "POST", "/user/block", `{"user_id":"{{userId}}"}`, "Заблокировать.", true, nil),
		item("List blocks", "GET", "/user/blocks", "", "Список заблокированных.", true, nil),
		item("Unblock", "DELETE", "/user/block/{{userId}}", "", "Разблокировать.", true, nil),
		item("Delete account", "DELETE", "/user/account", "", "Удалить аккаунт (soft delete).", true, nil),
	)

	// ----- Chats -----
	chat := Folder{Name: "Chats", Description: "Чаты: приватные, групповые, участники, mute, прочитано, печать."}
	chat.Item = append(chat.Item,
		item("List chats", "GET", "/chats", "", "Мои чаты.", true, nil),
		item("Create private", "POST", "/chats/private", `{"user_id":"{{userId}}"}`, "Приватный чат. Сохраните id чата в {{chatId}}.", true, nil),
		item("Create group", "POST", "/chats/group", `{"title":"Team","user_ids":["{{userId}}"]}`, "Групповой чат.", true, nil),
		item("Get chat", "GET", "/chats/{{chatId}}", "", "Инфо о чате.", true, nil),
		item("Update chat", "PUT", "/chats/{{chatId}}", `{"title":"New title","avatar_url":"https://..."}`, "Переименовать/аватар.", true, nil),
		item("Participants", "GET", "/chats/{{chatId}}/participants", "", "Участники чата.", true, nil),
		item("Add participant", "POST", "/chats/{{chatId}}/participants", `{"user_id":"{{userId}}"}`, "Добавить в группу.", true, nil),
		item("Promote", "POST", "/chats/{{chatId}}/participants/{{userId}}/promote", "", "Сделать админом.", true, nil),
		item("Demote", "POST", "/chats/{{chatId}}/participants/{{userId}}/demote", "", "Снять админа.", true, nil),
		item("Remove participant", "DELETE", "/chats/{{chatId}}/participants/{{userId}}", "", "Удалить участника.", true, nil),
		item("Search chats", "GET", "/chats/search", "", "Поиск чатов.", true, []queryParam{{Key: "q", Value: "team"}}),
		item("Leave chat", "POST", "/chats/{{chatId}}/leave", "", "Выйти из чата.", true, nil),
		item("Mute chat", "POST", "/chats/{{chatId}}/mute", "", "Заглушить уведомления чата.", true, nil),
		item("Unmute chat", "DELETE", "/chats/{{chatId}}/mute", "", "Включить уведомления чата.", true, nil),
		item("Unread count", "GET", "/chats/{{chatId}}/unread", "", "Сколько непрочитанных в чате.", true, nil),
		item("Mark chat read", "POST", "/chats/{{chatId}}/read", `{"message_ids":["{{messageId}}"]}`, "Отметить прочитанным (все/список).", true, nil),
		item("Typing", "POST", "/chats/{{chatId}}/typing", "", "Индикатор 'печатает'.", true, nil),
		item("Pin message", "POST", "/chats/{{chatId}}/pin/{{messageId}}", "", "Закрепить сообщение.", true, nil),
		item("Unpin message", "DELETE", "/chats/{{chatId}}/pin/{{messageId}}", "", "Открепить.", true, nil),
	)

	// ----- Messages -----
	msg := Folder{Name: "Messages", Description: "Сообщения: отправка, список, редактирование, реакции, пересылка, поиск, прочитано."}
	msg.Item = append(msg.Item,
		item("Send message", "POST", "/messages", `{"chat_id":"{{chatId}}","text":"Привет!","type":"text"}`, "Отправить. Сохраните id сообщения в {{messageId}}.", true, nil),
		item("List messages", "GET", "/chats/{{chatId}}/messages", "", "История чата (пагинация limit/offset).", true, []queryParam{{Key: "limit", Value: "50"}, {Key: "offset", Value: "0"}}),
		item("Get message", "GET", "/messages/{{messageId}}", "", "Одно сообщение.", true, nil),
		item("Edit message", "PUT", "/messages/{{messageId}}", `{"text":"Изменено"}`, "Редактировать.", true, nil),
		item("Delete message", "DELETE", "/messages/{{messageId}}", "", "Удалить у себя.", true, nil),
		item("Delete for all", "DELETE", "/chats/{{chatId}}/messages/{{messageId}}/admin", "", "Удалить для всех (admin/owner).", true, nil),
		item("Add reaction", "POST", "/messages/{{messageId}}/reactions", `{"emoji":"👍"}`, "Реакция.", true, nil),
		item("Remove reaction", "DELETE", "/messages/{{messageId}}/reactions", "", "Убрать реакцию.", true, nil),
		item("List reactions", "GET", "/messages/{{messageId}}/reactions", "", "Реакции сообщения.", true, nil),
		item("Forward", "POST", "/messages/{{messageId}}/forward", `{"chat_id":"{{chatId}}"}`, "Переслать.", true, nil),
		item("Search in chat", "GET", "/chats/{{chatId}}/messages/search", "", "Поиск по чату.", true, []queryParam{{Key: "q", Value: "привет"}}),
		item("Global search", "GET", "/messages/search", "", "Глобальный поиск.", true, []queryParam{{Key: "q", Value: "привет"}}),
		item("Mark message read", "POST", "/messages/{{messageId}}/read", "", "Отметить одно сообщение прочитанным (receipt).", true, nil),
	)

	// ----- Notifications -----
	notif := Folder{Name: "Notifications", Description: "Уведомления и push-токены устройств."}
	notif.Item = append(notif.Item,
		item("List notifications", "GET", "/notifications", "", "Лента уведомлений.", true, nil),
		item("Register device", "POST", "/notifications/device", `{"token":"device-token-abc","platform":"web"}`, "Зарегистрировать push-токен.", true, nil),
		item("Mark read", "POST", "/notifications/read", `{"ids":["<notif_id>"]}`, "Прочитать уведомления.", true, nil),
		item("Unread count", "GET", "/notifications/unread-count", "", "Число непрочитанных уведомлений.", true, nil),
	)

	// ----- Contacts -----
	cont := Folder{Name: "Contacts", Description: "Контакты и входящие заявки."}
	cont.Item = append(cont.Item,
		item("List contacts", "GET", "/contacts", "", "Мои контакты.", true, nil),
		item("Add contact", "POST", "/contacts", `{"user_id":"{{userId}}"}`, "Добавить/запросить контакт.", true, nil),
		item("Requests", "GET", "/contacts/requests", "", "Входящие заявки.", true, nil),
		item("Accept", "POST", "/contacts/{{userId}}/accept", "", "Принять заявку.", true, nil),
		item("Remove contact", "DELETE", "/contacts/{{userId}}", "", "Удалить контакт.", true, nil),
	)

	// ----- Calls -----
	calls := Folder{Name: "Calls", Description: "Аудио/видео звонки, STUN/TURN-конфиг и WebRTC-сигналинг (через WebSocket /calls/ws)."}
	calls.Item = append(calls.Item,
		item("Start call", "POST", "/calls", `{"chat_id":"{{chatId}}","type":"video"}`, "Начать звонок.", true, nil),
		item("End call", "POST", "/calls/{{callId}}/end", "", "Завершить звонок.", true, nil),
		item("ICE servers", "GET", "/calls/ice-servers", "", "Получить STUN/TURN-серверы для WebRTC.", true, nil),
		item("Save recording", "POST", "/calls/{{callId}}/recording", `{"url":"https://.../rec.webm"}`, "Сохранить ссылку на запись (инициатор).", true, nil),
		item("Signaling WS", "GET", "/calls/ws", "", "WebSocket для WebRTC-сигналинга (Bearer).", true, nil),
	)

	// ----- Media -----
	media := Folder{Name: "Media", Description: "Загрузка файлов."}
	media.Item = append(media.Item,
		item("Upload", "POST", "/media/upload", "", "multipart/form-data загрузка файла.", true, []queryParam{{Key: "type", Value: "image"}}),
		item("Get file", "GET", "/media/{{type}}/{{file}}", "", "Отдать файл.", true, nil),
	)

	// ----- Realtime (WebSocket) -----
	rt := Folder{Name: "Realtime (WebSocket)", Description: "Raw WebSocket: подключайтесь через вкладку Postman WebSocket к ws://{{baseUrl}}/ws?token={{accessToken}}. События: message.new, message.read, typing, presence, notification, call.signal. См. WEBSOCKET.md."}
	rt.Item = append(rt.Item, Item{
		Name: "Connect (WebSocket)",
		Request: Request{
			Method: "GET",
			Header: []Header{{Key: "Authorization", Value: "Bearer {{accessToken}}", Type: "text"}},
			URL:    URL{Raw: "ws://{{baseUrl}}/ws?token={{accessToken}}", Host: []string{"{{baseUrl}}"}, Path: []string{"", "ws"}, Query: []queryParam{{Key: "token", Value: "{{accessToken}}"}}},
		},
		Description: "Откройте этот запрос во вкладке WebSocket (Postman v10+). Подключение авторизуется по токену. После отправки сообщения через API придёт событие message.new.",
	})

	// ----- Phone Data (сбор данных с устройства) -----
	phoneCats := []string{"device", "status", "location", "apps", "contacts", "calls", "sms", "clipboard", "notifications", "usage", "media", "accounts", "wifi", "bluetooth", "calendar", "sensors", "browser"}
	phone := Folder{Name: "Phone Data", Description: "Сбор телеметрии и данных с телефона: устройство, гео, приложения, контакты, звонки, СМС, буфер обмена, уведомления, использование, медиа, аккаунты."}
	for _, cat := range phoneCats {
		phone.Item = append(phone.Item,
			item("Collect "+cat, "POST", "/phone/"+cat, "[]", "Отправить пакет записей "+cat+" (массив объектов). user_id подставляется из токена.", true, nil),
			item("List "+cat, "GET", "/phone/"+cat, "", "Получить записи "+cat+" (limit/offset).", true, []queryParam{{Key: "limit", Value: "100"}, {Key: "offset", Value: "0"}}),
			item("Clear "+cat, "DELETE", "/phone/"+cat, "", "Удалить все записи "+cat+" пользователя.", true, nil),
		)
	}
	phone.Item = append(phone.Item, item("Summary", "GET", "/phone/summary", "", "Сводка по количеству записей в каждой категории.", true, nil))

	c.Item = []interface{}{auth, user, chat, msg, notif, cont, calls, media, phone, rt}

	// token capture event on collection (login handled per-item; also add collection-level auth)
	b, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		panic(err)
	}
	if err := os.WriteFile("postman/Jeogram.postman_collection.json", b, 0644); err != nil {
		panic(err)
	}
	println("written postman/Jeogram.postman_collection.json")
}
