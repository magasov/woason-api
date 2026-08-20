# WOAson API

Бэкенд маркетплейса **WOAson** (ВОАЗОН — «всё в одной зоне»): REST + PostgreSQL + WebSocket + ЮKassa + админ-API.

Админка — только JSON-ручки `/api/v1/admin/*` под JWT с ролью `admin`. HTML/SPA/дашборда нет.

## Адреса

| Среда | URL |
| --- | --- |
| API | https://api.woason.ru |
| Документация | https://api.woason.ru/docs |
| Витрина (фронт) | https://woason.ru |
| WebSocket | `wss://api.woason.ru/ws?token=ACCESS_JWT` |
| Локально | `http://localhost:8080` · фронт `http://localhost:3000` |

## Стек

- Go 1.22+, chi, pgx, golang-migrate, JWT, gorilla/websocket
- PostgreSQL 16
- ЮKassa API v3, тестовый ключ `test_*`
- Прод: Docker Compose + Caddy (TLS) на VPS

## Запуск локально

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

CORS открыт для `http://localhost:3000` и `https://woason.ru`.

### Переменные окружения

См. `.env.example`. Секрет ЮKassa в git не коммитится (файл `.env` в `.gitignore`).

| Ключ | Назначение |
| --- | --- |
| `DATABASE_URL` | PostgreSQL |
| `JWT_SECRET` | подпись access/refresh |
| `YKASSA_SHOP_ID` / `YKASSA_SECRET_KEY` / `YKASSA_API_URL` | тестовые платежи |
| `YKASSA_RETURN_URL` | редирект после оплаты (`https://woason.ru/orders/:id`) |
| `PAYMENTS_MOCK` | `true` — сразу `paid` без ЮKassa (только dev) |
| `ADMIN_EMAIL` / `ADMIN_PASSWORD` | создаётся только админ, каталог пустой |
| `PUBLIC_BASE_URL` | публичный адрес API (`https://api.woason.ru`) |
| `FRONTEND_URL` | витрина (`https://woason.ru`) |

При старте сидится только админ из `.env`. Каталог пустой: магазины и товары создают продавцы через API. 22 товарные категории (как WB/Ozon) отдаются списком; классифайд (авто, недвижимость, услуги, вакансии, туры) не отдаётся и не создаётся.

Админ — единственная учётка из `.env`: `ADMIN_EMAIL` / `ADMIN_PASSWORD` (по умолчанию `admin@woason.ru` / `123456`).

## Примеры curl

Логин админа:

```bash
curl -s https://api.woason.ru/api/v1/auth/login \
  -H 'Content-Type: application/json' \
  -d '{"email":"admin@woason.ru","password":"123456"}'
```

Админ-статистика:

```bash
TOKEN=$(curl -s https://api.woason.ru/api/v1/auth/login \
  -H 'Content-Type: application/json' \
  -d '{"email":"admin@woason.ru","password":"123456"}' | jq -r .accessToken)

curl -s https://api.woason.ru/api/v1/admin/stats \
  -H "Authorization: Bearer $TOKEN"
```

Регистрация покупателя и продавца — `POST /api/v1/auth/register`. Товары продавец добавляет через `POST /api/v1/seller/products`.

Ответ checkout: `{ "order": {...}, "confirmationUrl": "https://..." }`.  
Webhook ЮKassa: `POST https://api.woason.ru/api/v1/payments/yookassa/webhook` (без JWT).

WebSocket: `wss://api.woason.ru/ws?token=ACCESS_JWT`

```json
{"type":"subscribe","channel":"chat","peerId":"<id собеседника>"}
{"type":"chat.send","peerId":"<id собеседника>","text":"Здравствуйте"}
```

## Прод (VPS)

Сервер: Docker Compose (`docker-compose.prod.yml`) + Caddy.

```bash
cd /opt/woason/api
docker compose -f docker-compose.prod.yml --env-file .env up -d --build
```

Фронт кладётся в `deploy/frontend/` — см. [deploy/FRONTEND.md](deploy/FRONTEND.md).

CI/CD: GitHub Actions (`.github/workflows/ci.yml`) — тесты на каждый push/PR, выкладка на VPS с `main` по SSH.

Секреты репозитория: `SSH_HOST`, `SSH_USER`, `SSH_PRIVATE_KEY`.

## Тесты

```bash
make test
```

Нужен доступный Postgres (`DATABASE_URL` из `.env`). Без БД пропускаются интеграционные тесты; расчёт доставки проверяется всегда.

## Документация API

- https://api.woason.ru/docs
- https://api.woason.ru/docs/openapi.yaml
- https://api.woason.ru/docs/terms

Локально: `http://localhost:8080/docs`

## Префикс API

`/api/v1` · health: `GET /health` · docs: `GET /docs`
