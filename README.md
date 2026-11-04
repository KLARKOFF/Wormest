# Wormest

Личная база знаний: заметки, книги и документы с обычным и семантическим поиском.
Self-hosted — сервер один, ноутбук и телефон открывают его в браузере, поэтому данные всегда в одном месте.

## Запуск через Docker

```sh
cp .env.example .env      # поменяйте POSTGRES_PASSWORD
docker compose up -d --build
```

Приложение откроется на `http://localhost:8000`. С телефона — `http://<IP компьютера>:8000`
(IP можно узнать командой `ip addr`). Миграции накатываются сами при старте сервера.

## Режим разработки

Нужны Go 1.26+ и Node.js 22+.

```sh
# 1. Только база, с открытым портом
docker compose -f docker-compose.yml -f docker-compose.dev.yml up -d db

# 2. Бэкенд (пароль — как в .env)
cd backend
DATABASE_URL='postgres://wormest:ПАРОЛЬ@localhost:5432/wormest?sslmode=disable' go run ./cmd/server

# 3. Фронтенд, в другом терминале
cd frontend
npm install
npm run dev
```

Vite откроется на `http://localhost:5173` и проксирует `/api` на `:8080`.

## Структура

```
backend/
  cmd/server/main.go          точка входа: конфиг → база → миграции → HTTP
  internal/config/            настройки из переменных окружения
  internal/db/                пул соединений и миграции (goose, вшиты в бинарник)
  internal/db/migrations/     SQL-схема
  internal/store/             все SQL-запросы
  internal/api/               маршруты, проверка входа, хендлеры
frontend/
  src/api.ts                  клиент REST API и типы
  src/auth.tsx                состояние входа
  src/pages/                  экраны: вход, список, редактор
  src/styles.css              оформление и токены цветов
  nginx.conf                  раздача статики и прокси /api в Docker
```

Новая миграция — новый файл `backend/internal/db/migrations/0000N_что_меняем.sql`.
Старые миграции не редактируются.

## REST API

| Метод | Путь | Что делает |
|---|---|---|
| POST | `/api/auth/register` | `{email, password, displayName}` → пользователь + кука |
| POST | `/api/auth/login` | `{email, password}` → пользователь + кука |
| POST | `/api/auth/logout` | завершить сессию |
| GET | `/api/auth/me` | текущий пользователь или 401 |
| GET | `/api/notes?tag=` | список заметок, свежие сверху |
| POST | `/api/notes` | `{title, body, tags}` → новая заметка |
| GET | `/api/notes/{id}` | заметка целиком |
| PUT | `/api/notes/{id}` | `{title, body, tags, version}` → 200, или 409 с актуальной версией |
| DELETE | `/api/notes/{id}` | удалить (только владелец) |
| GET | `/api/tags` | теги с количеством заметок |
| GET | `/api/health` | проверка, что сервер жив |

Ошибки приходят как `{"error": "текст для пользователя"}`.