<p align="center">
  <img src="../assets/hdr-admin.svg" width="100%" alt="Сбор данных с телефона"/>
</p>
# Сбор данных с телефона

Префикс: `/phone`. Все эндпоинты требуют `Bearer`. Данные привязываются к
`user_id` из JWT (значение из тела запроса игнорируется — защита от подмены).

Каждая категория поддерживает три метода:

- `POST /phone/{category}` — отправить **массив** записей.
- `GET /phone/{category}?limit=100&offset=0` — получить записи.
- `DELETE /phone/{category}` — удалить все записи пользователя.

## Категории

| Категория | Назначение |
| --- | --- |
| `device` | информация об устройстве |
| `status` | батарея, заряд, сеть |
| `location` | геолокация |
| `apps` | установленные приложения |
| `contacts` | контакты телефона |
| `calls` | журнал звонков |
| `sms` | СМС |
| `clipboard` | буфер обмена |
| `notifications` | перехваченные уведомления |
| `usage` | статистика использования приложений |
| `media` | медиафайлы |
| `accounts` | аккаунты на устройстве |
| `wifi` | Wi-Fi сети (`ssid`, `bssid`, `signal_level`, `security_type`) |
| `bluetooth` | Bluetooth-устройства |
| `calendar` | события календаря |
| `sensors` | показания датчиков |
| `browser` | история браузера |

## Сводка

`GET /phone/summary` — количество записей по каждой категории:

```json
{
  "success": true,
  "data": { "wifi": 1, "calls": 3, "sms": 12, "...": 0 }
}
```

Пример отправки Wi-Fi:

```bash
curl -X POST http://localhost:8080/phone/wifi \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '[{"ssid":"HomeNet","bssid":"aa:bb:cc","signal_level":-50,"security_type":"WPA2"}]'
```
