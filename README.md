# URL Shortener

Сервис коротких ссылок: бэкенд на Go (REST, PostgreSQL) и фронтенд на React.

## Запуск

Нужен только Docker.

```bash
cp .env.example .env # и задать свой POSTGRES_PASSWORD
docker compose up -d --build
```

Приложение: http://localhost:3000

Порядок старта: `postgres` → `migrate` (накатывает миграции и завершается) → `backend` → `frontend`.

## API

Контракт - [backend/api/openapi.yaml](backend/api/openapi.yaml). Сервер генерируется из него (`oapi-codegen`),
запросы валидируются по нему же. Сгенерированный код (`backend/generated/`) и моки (`mocks/`) в репозиторий
не коммитятся: Docker-образ генерирует сервер сам, а локально их создаёт `go generate ./...`.

| Метод | Путь | Описание |
|---|---|---|
| `GET` | `/api/v1/links?limit=20&offset=0` | Список ссылок (новые сверху) и общее количество |
| `POST` | `/api/v1/links` | Создать: `{"originalUrl": "https://...", "code": "необязательно"}` |
| `GET` | `/api/v1/links/{id}` | Получить ссылку |
| `PATCH` | `/api/v1/links/{id}` | Изменить `originalUrl` и/или `code` |
| `DELETE` | `/api/v1/links/{id}` | Удалить |
| `GET` | `/r/{code}` | Перейти по короткой ссылке (302) и увеличить счётчик |
| `GET` | `/healthz` | Liveness: процесс жив |
| `GET` | `/readyz` | Readiness: есть соединение с базой |

Ошибки: `{"error": {"code": "not_found|code_taken|validation_error|bad_request|internal_error", "message": "..."}}`.

## Конфигурация бэкенда

Бинарник - CLI с командами `serve` (по умолчанию) и `migrate`, вся конфигурация - через переменные окружения.

| Переменная | По умолчанию | |
|---|---|---|
| `HTTP_PORT` | `8080` | Порт HTTP-сервера |
| `SHUTDOWN_TIMEOUT` | `10s` | Сколько ждать завершения запросов после SIGTERM |
| `LOG_LEVEL` | `info` | `debug`, `info`, `warn`, `error` |
| `POSTGRES_HOST` / `POSTGRES_PORT` / `POSTGRES_DB` | `localhost` / `5432` / `shortener` | |
| `POSTGRES_USER` / `POSTGRES_PASSWORD` | - | Обязательные |
| `POSTGRES_SSLMODE` | `disable` | |
| `POSTGRES_MAX_CONNS` | `10` | Размер пула соединений |

Фронтенд (nginx): `BACKEND_URL` - адрес бэкенда, по умолчанию `http://backend:8080`.
