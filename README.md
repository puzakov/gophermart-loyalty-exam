# gophermart-loyalty-exam

HTTP API накопительной системы лояльности «Гофермарт».

## Конфигурация

Поддерживаются переменные окружения и флаги:

- `RUN_ADDRESS` или `-a` — адрес/порт запуска сервиса (по умолчанию `localhost:8080`)
- `DATABASE_URI` или `-d` — DSN PostgreSQL
- `ACCRUAL_SYSTEM_ADDRESS` или `-r` — базовый URL accrual-системы (по умолчанию `http://localhost:8081`)

Также используются:

- `JWT_SECRET` — секрет подписи access JWT
- `JWT_ACCESS_TTL` — TTL access-токена (по умолчанию `30*24h`, то есть 1 месяц)
- `ACCRUAL_WORKERS` — число воркеров в пуле
- `ACCRUAL_POLL_INTERVAL` — период опроса очереди заказов (например `2s`)

Для разработки можно скопировать `.env.example` в `.env`.

## Запуск (локально)

1) Поднять PostgreSQL (2 базы) через compose:

```bash
docker compose up -d
```

2) Запустить accrual-систему (отдельный терминал):

```bash
export RUN_ADDRESS=localhost:8081
export DATABASE_URI="postgres://accrual_db_user:secret@localhost:5435/accrual_db_app?sslmode=disable"
./cmd/accrual/accrual_darwin_arm64
```

3) Запустить gophermart:

```bash
go run ./cmd/gophermart -a localhost:8080 -d "postgres://gophermart_db_user:secret@localhost:5434/gophermart_db_app?sslmode=disable" -r "http://localhost:8081"
```

Миграции выполняются автоматически при старте (папка `migrations/`).

## Авторизация

Используется `Authorization: Bearer <access_token>`.

`POST /api/user/register` и `POST /api/user/login` возвращают:

```json
{ "access_token": "..." }
```

В дополнение сервер выставляет заголовок ответа `Authorization: Bearer <access_token>`.

## Документация API

- `docs/REST.md` — расширенное описание REST (примеры, коды, правила)
- `docs/openapi.yaml` — OpenAPI 3.0 спецификация
