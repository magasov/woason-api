# WOAson API

Бэкенд маркетплейса **WOAson** (ВОАЗОН — «всё в одной зоне»): REST + PostgreSQL + WebSocket + ЮKassa (тест) + админ-API.

Админка — только JSON-ручки `/api/v1/admin/*` под JWT с ролью `admin`. HTML/SPA/дашборда нет.

## Стек

- Go 1.22+, chi, pgx, golang-migrate, JWT, gorilla/websocket
- PostgreSQL 16
- ЮKassa API v3, тестовый ключ `test_*`

## Запуск

```bash
cp .env.example .env   # затем заполните YKASSA_SECRET_KEY
docker compose up -d postgres
make migrate
make run
```

Или всё сразу:

```bash
docker compose up --build
```

API: `http://localhost:8080`  
Фронт покупателя/продавца: `http://localhost:3000` (CORS уже открыт)

### Переменные окружения

См. `.env.example`. Секрет ЮKassa в git не коммитится (файл `.env` в `.gitignore`).

| Ключ | Назначение |
| --- | --- |
| `DATABASE_URL` | PostgreSQL |
| `JWT_SECRET` | подпись access/refresh |
| `YKASSA_SHOP_ID` / `YKASSA_SECRET_KEY` / `YKASSA_API_URL` | тестовые платежи |
| `YKASSA_RETURN_URL` | редирект после оплаты (`/orders/:id`) |
| `PAYMENTS_MOCK` | `true` — сразу `paid` без ЮKassa (только dev) |
| `ADMIN_EMAIL` / `ADMIN_PASSWORD` | создаётся только админ, каталог пустой |

При старте сидится только админ из `.env`. Каталог пустой: магазины и товары создают продавцы через API. 22 товарные категории (как WB/Ozon) отдаются списком; классифайд (авто, недвижимость, услуги, вакансии, туры) не отдаётся и не создаётся.

Админ — единственная учётка из `.env`: `ADMIN_EMAIL` / `ADMIN_PASSWORD` (по умолчанию `admin@woason.ru` / `123456`).

## Примеры curl

Логин админа:

```bash
curl -s http://localhost:8080/api/v1/auth/login \
  -H 'Content-Type: application/json' \
  -d '{"email":"admin@woason.ru","password":"123456"}'
```

Админ-статистика:

```bash
TOKEN=$(curl -s http://localhost:8080/api/v1/auth/login \
  -H 'Content-Type: application/json' \
  -d '{"email":"admin@woason.ru","password":"123456"}' | jq -r .accessToken)

curl -s http://localhost:8080/api/v1/admin/stats \
  -H "Authorization: Bearer $TOKEN"
```

Регистрация покупателя и продавца — `POST /api/v1/auth/register`. Товары продавец добавляет через `POST /api/v1/seller/products`.

Ответ checkout: `{ "order": {...}, "confirmationUrl": "https://..." }`.  
Webhook ЮKassa: `POST /api/v1/payments/yookassa/webhook` (без JWT).

WebSocket: `ws://localhost:8080/ws?token=ACCESS_JWT`

```json
{"type":"subscribe","channel":"chat","peerId":"<id собеседника>"}
{"type":"chat.send","peerId":"<id собеседника>","text":"Здравствуйте"}
```

## Тесты

```bash
make test
```

Нужен доступный Postgres (`DATABASE_URL` из `.env`). Без БД пропускаются интеграционные тесты; расчёт доставки проверяется всегда.

## Префикс API

`/api/v1` · health: `GET /health`
