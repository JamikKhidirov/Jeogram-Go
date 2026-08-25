#!/usr/bin/env python3
"""
Генератор ПОНЯТНОЙ Postman-коллекции для Jeogram.

Результат:
  postman/Jeogram API.postman_collection.json  - все HTTP + WebSocket эндпоинты по папкам
  postman/Jeogram.postman_environment.json     - переменные (base_url, токены, id и т.п.)

Источник истины - docs/swagger.json (swagger 2.0), обновляется через `make swagger`.
После генерации импортируйте ОБА файла в Postman (Environment -> сначала окружение,
затем коллекцию). Все запросы наследуют Bearer-токен из переменной {{access_token}}.
"""
import json
import os

ROOT = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
SWAGGER = os.path.join(ROOT, "docs", "swagger.json")
OUT_COLLECTION = os.path.join(ROOT, "postman", "Jeogram API.postman_collection.json")
OUT_ENV = os.path.join(ROOT, "postman", "Jeogram.postman_environment.json")

# Эндпоинты, НЕ требующие авторизации (публичные).
PUBLIC_PREFIXES = (
    "/auth/register",
    "/auth/login",
    "/auth/refresh",
    "/auth/request-otp",
    "/auth/verify-otp",
    "/auth/forgot-password",
    "/auth/reset-password",
    "/auth/2fa/verify",
)

# Это НЕ HTTP-эндпоинты, а WebSocket-соединения. Они описаны в Swagger как GET
# только для документации, поэтому из HTTP-папок их исключаем и выносим в
# отдельную папку Realtime как настоящие WebSocket-запросы.
WS_PATHS = {"/ws", "/socket.io/", "/calls/ws"}


def load_swagger():
    with open(SWAGGER, encoding="utf-8") as f:
        return json.load(f)


def smart_default(name):
    n = name.lower()
    if "email" in n:
        return "user@example.com"
    if "password" in n:
        return "Passw0rd!23"
    if "username" in n or "user_id" in n or n == "user":
        return "user"
    if "phone" in n:
        return "+79001234567"
    if "code" in n or "otp" in n:
        return "123456"
    if "token" in n:
        return "{{access_token}}"
    if "url" in n:
        return "https://example.com/file.jpg"
    if "title" in n:
        return "My Chat"
    if "text" in n:
        return "Hello!"
    if "name" in n:
        return "name"
    if "status" in n:
        return "active"
    if "type" in n:
        return "text"
    if "id" in n:
        return "00000000-0000-0000-0000-000000000000"
    return "string"


def sample(defs, schema, depth=0):
    if depth > 6:
        return None
    if "$ref" in schema:
        name = schema["$ref"].split("/")[-1]
        return sample(defs, defs.get(name, {}), depth + 1)
    t = schema.get("type")
    if t == "object" or "properties" in schema:
        props = schema.get("properties", {})
        return {k: sample(defs, v, depth + 1) for k, v in props.items()}
    if t == "array":
        return [sample(defs, schema.get("items", {}), depth + 1)]
    if t == "integer":
        return 0
    if t == "number":
        return 0.0
    if t == "boolean":
        return True
    if t == "string":
        return smart_default(schema.get("x-go-name", schema.get("example", "x")))
    return None


def build_url(path, operation):
    # path params -> {{name}}, query params -> ?k={{k}}
    url = "{{base_url}}" + path
    url = url.replace("{", "{{").replace("}", "}}")
    queries = [p for p in operation.get("parameters", []) if p.get("in") == "query"]
    if queries:
        parts = []
        for q in queries:
            parts.append("%s={{%s}}" % (q["name"], q["name"]))
        url += "?" + "&".join(parts)
    return url


def build_body(defs, operation):
    for p in operation.get("parameters", []):
        if p.get("in") == "body":
            schema = p.get("schema", {})
            data = sample(defs, schema)
            if data is not None:
                return json.dumps(data, ensure_ascii=False, indent=2)
    return None


def make_item(defs, method, path, operation):
    if path in WS_PATHS:
        return None
    name = operation.get("summary") or (method.upper() + " " + path)
    url = build_url(path, operation)
    body = build_body(defs, operation)
    headers = []
    is_public = any(path.startswith(p) for p in PUBLIC_PREFIXES)
    if not is_public:
        headers.append({"key": "Authorization", "value": "Bearer {{access_token}}", "type": "text"})
    headers.append({"key": "Content-Type", "value": "application/json", "type": "text"})
    req = {
        "method": method.upper(),
        "header": headers,
        "url": url,
        "description": operation.get("description", ""),
    }
    if body:
        req["body"] = {"mode": "raw", "raw": body,
                       "options": {"raw": {"language": "json"}}}
    item = {
        "name": name,
        "request": req,
    }
    # Для публичных эндпоинтов отключаем наследуемую Bearer-авторизацию.
    if is_public:
        item["auth"] = {"type": "noauth"}
    # Test-скрипт: захват токенов из ответа в переменные коллекции.
    captures = []
    if path in ("/auth/login", "/auth/2fa/verify"):
        captures.append(
            "var d = pm.response.json().data || {};"
            "if (d.access_token) pm.collectionVariables.set('access_token', d.access_token);"
            "if (d.refresh_token) pm.collectionVariables.set('refresh_token', d.refresh_token);"
        )
    if path == "/auth/register":
        captures.append(
            "var d = pm.response.json().data || {};"
            "if (d.access_token) pm.collectionVariables.set('access_token', d.access_token);"
        )
    if path == "/auth/refresh":
        captures.append(
            "var d = pm.response.json().data || {};"
            "if (d.access_token) pm.collectionVariables.set('access_token', d.access_token);"
        )
    if captures:
        item["event"] = [{"listen": "test", "script": {"type": "text/javascript", "exec": captures}}]
    return item


def make_ws_items():
    # ВАЖНО: схема ws:// должна быть ЛИТЕРАЛЬНОЙ (а не в переменной), иначе Postman
    # при импорте посчитает это обычным HTTP GET-запросом. Хост берётся из
    # переменной {{ws_host}} (по умолчанию localhost:8080).
    ws_desc = (
        "ЭТО WEBSOCKET-ЗАПРОС, а не HTTP (Postman откроет вкладку WebSocket, не меняйте метод).\n"
        "Нажмите Connect. Токен передаётся в query-параметре token.\n"
        "После подключения сервер шлёт ping каждые 30с и push-события "
        '{"type":"<event>","payload":{...}} (например new_message, message_edited, call_invite, user_status).\n'
        "Примеры исходящих событий:\n"
        '{"type":"typing","chat_id":"{{chat_id}}"}\n'
        '{"type":"call.signal","chat_id":"{{chat_id}}","to":"{{user_id}}","payload":{}}'
    )
    ws = {
        "name": "1. WS /ws — подключение (raw WebSocket)",
        "request": {
            "method": "GET",
            "header": [],
            "url": "ws://{{ws_host}}/ws?token={{access_token}}",
            "description": ws_desc,
        },
        "protocolProfileBehavior": {"disableBodyPruning": True},
    }
    sio_desc = (
        "ЭТО WEBSOCKET-ЗАПРОС (Socket.IO v2), а не обычный HTTP GET.\n"
        "Подключение только для клиента socket.io-client@2.x. query-параметры: "
        "token + EIO=4 + transport=websocket.\n"
        "После handshake отправьте engine.io-фреймы: сначала '40' (probe), "
        "затем события в формате '42[\"message.new\",{...}]'. События идентичны /ws."
    )
    sio = {
        "name": "2. Socket.IO /socket.io — подключение (v2)",
        "request": {
            "method": "GET",
            "header": [],
            "url": "ws://{{ws_host}}/socket.io/?token={{access_token}}&EIO=4&transport=websocket",
            "description": sio_desc,
        },
        "protocolProfileBehavior": {"disableBodyPruning": True},
    }
    calls_ws_desc = (
        "ЭТО WEBSOCKET-ЗАПРОС (WebRTC-сигналинг), а не HTTP GET.\n"
        "Подключитесь к /calls/ws с токеном и шлите:\n"
        '{"type":"offer","chat_id":"{{chat_id}}","to":"{{user_id}}","payload":{}}'
    )
    calls_ws = {
        "name": "3. WS /calls/ws — сигналинг звонков (WebRTC)",
        "request": {
            "method": "GET",
            "header": [],
            "url": "ws://{{ws_host}}/calls/ws?token={{access_token}}",
            "description": calls_ws_desc,
        },
        "protocolProfileBehavior": {"disableBodyPruning": True},
    }
    return [ws, sio, calls_ws]


def main():
    sw = load_swagger()
    defs = sw.get("definitions", {})
    paths = sw.get("paths", {})

    folders = {}
    order = []
    for path, ops in paths.items():
        for method, op in ops.items():
            if method.lower() not in ("get", "post", "put", "delete", "patch"):
                continue
            item = make_item(defs, method, path, op)
            if item is None:
                continue
            tags = op.get("tags") or ["other"]
            tag = tags[0]
            if tag not in folders:
                folders[tag] = []
                order.append(tag)
            folders[tag].append(item)

    items = []
    # Предсказуемый порядок папок
    preferred = ["auth", "users", "contacts", "chats", "messages", "media",
                 "calls", "notifications", "phone", "admin", "realtime", "other"]
    tags_sorted = [t for t in preferred if t in folders] + [t for t in order if t not in preferred]
    for tag in tags_sorted:
        if not folders[tag]:
            continue
        items.append({
            "name": tag.capitalize(),
            "description": "Группа эндпоинтов: " + tag,
            "item": folders[tag],
        })
    # Папка Realtime/WebSocket всегда последняя и понятная
    items.append({
        "name": "Realtime (WebSocket / Socket.IO)",
        "description": "Подключения реального времени. См. описание каждого запроса.",
        "item": make_ws_items(),
    })

    collection = {
        "info": {
            "name": "Jeogram API",
            "description": (
                "Полная коллекция Jeogram Messenger. Импортируйте сначала файл окружения "
                "(Jeogram.postman_environment.json), выберите его в Postman (Environment), "
                "затем коллекцию. Войдите через /auth/login (или /auth/register) — токен "
                "автоматически сохранится в переменную access_token и подставится во все запросы.\n\n"
            "Переменные: {{base_url}}=http://localhost:8080, {{ws_host}}=localhost:8080 (для WebSocket).\n"
            "Для путей с {id}/{chat_id} и т.п. заполните соответствующие переменные коллекции.\n"
            "WebSocket-запросы находятся в папке 'Realtime (WebSocket / Socket.IO)' и имеют "
            "литеральную схему ws:// — Postman откроет их как WebSocket, а не HTTP GET."
            ),
            "schema": "https://schema.getpostman.com/json/collection/v2.1.0/collection.json",
        },
        "auth": {"type": "bearer", "bearer": [{"key": "token", "value": "{{access_token}}", "type": "string"}]},
        "item": items,
        "variable": [
            {"key": "base_url", "value": "http://localhost:8080"},
            {"key": "ws_host", "value": "localhost:8080"},
            {"key": "access_token", "value": ""},
            {"key": "refresh_token", "value": ""},
            {"key": "chat_id", "value": ""},
            {"key": "message_id", "value": ""},
            {"key": "call_id", "value": ""},
            {"key": "user_id", "value": ""},
        ],
    }

    os.makedirs(os.path.dirname(OUT_COLLECTION), exist_ok=True)
    with open(OUT_COLLECTION, "w", encoding="utf-8") as f:
        json.dump(collection, f, ensure_ascii=False, indent=2)

    env = {
        "name": "Jeogram Local",
        "values": [
            {"key": "base_url", "value": "http://localhost:8080", "enabled": True},
            {"key": "ws_host", "value": "localhost:8080", "enabled": True},
            {"key": "access_token", "value": "", "enabled": True},
            {"key": "refresh_token", "value": "", "enabled": True},
            {"key": "chat_id", "value": "", "enabled": True},
            {"key": "message_id", "value": "", "enabled": True},
            {"key": "call_id", "value": "", "enabled": True},
            {"key": "user_id", "value": "", "enabled": True},
        ],
        "_postman_variable_scope": "environment",
    }
    with open(OUT_ENV, "w", encoding="utf-8") as f:
        json.dump(env, f, ensure_ascii=False, indent=2)

    print("written", OUT_COLLECTION, "and", OUT_ENV)
    print("folders:", tags_sorted + ["Realtime (WebSocket / Socket.IO)"])


if __name__ == "__main__":
    main()
