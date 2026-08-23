# Развёртывание в Kubernetes

Манифесты в этой папке поднимают весь стек Jeogram в namespace `jeogram`.

## Что внутри
- `00-namespace.yaml` — namespace `jeogram`.
- `01-secrets.yaml` — секреты приложения (JWT, пароль БД, ключи push). **Замените
  значения на свои перед применением!**
- `02-postgres.yaml` — StatefulSet + PVC + Service (PostgreSQL 16).
- `03-redis.yaml` — Deployment + Service (Redis 7).
- `04-kafka.yaml` — одиночный брокер Kafka (KRaft). Опционален: если не нужен,
  удалите файл и поставьте `KAFKA_ENABLED=false` в `05-configmap.yaml`.
- `05-configmap.yaml` — все настройки приложения (без секретов).
- `06-app-deployment.yaml` — Deployment (2 реплики) + PVC для загрузок.
- `07-app-service.yaml` — ClusterIP Service (http/ws).
- `08-ingress.yaml` — Ingress для `jeogram.local` (с настройками WebSocket).
- `kustomization.yaml` — для `kubectl apply -k k8s/`.

## Откуда брать значения для секретов
- `postgres-password`, `jwt-access-secret`, `jwt-refresh-secret` — придумывайте/генерируйте
  сами (например, `openssl rand -base64 48`). Это НЕ внешние ключи.
- `fcm-server-key` — из Firebase Console (Project Settings → Cloud Messaging → Server key).
- `apns-key` — содержимое `.p8`-ключа из Apple Developer (Keys → APNs).
- `apns-key` в манифесте — это строка; для файла используйте `APNS_KEY_PATH` и смонтируйте
  секрет как файл в поде (в примере путь `/data/apns/authkey.p8`).

## Быстрый старт
```bash
# 1) Создайте секреты (или отредактируйте 01-secrets.yaml)
kubectl -n jeogram create secret generic jeogram-secrets \
  --from-literal=postgres-password=<ПАРОЛЬ> \
  --from-literal=jwt-access-secret=<СЕКРЕТ> \
  --from-literal=jwt-refresh-secret=<СЕКРЕТ> \
  --from-literal=fcm-server-key=<FCM> \
  --from-literal=apns-key=<P8>

# 2) Применить всё
kubectl apply -k k8s/

# 3) Проверить
kubectl -n jeogram get pods
kubectl -n jeogram port-forward svc/jeogram-app 8080:80
# затем откройте http://localhost:8080/swagger/index.html
```

## Сборка образа
```bash
docker build -t jeogram/messenger:latest .
# при использовании своего реестра:
docker tag jeogram/messenger:latest <registry>/jeogram/messenger:latest
docker push <registry>/jeogram/messenger:latest
# и обновите image: в 06-app-deployment.yaml
```
