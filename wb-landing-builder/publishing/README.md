# Компонент публикаций (publishing)

Компонент отвечает за публикацию черновиков лендингов: берёт актуальный снимок из **storage**, собирает bundle файлов и сохраняет его в S3-совместимое хранилище (локально — MinIO). Метаданные публикации хранятся в MongoDB.

В монолите это реализация идеи **Publishing Service** из ADR «Сервис публикации лендингов» — отдельный микросервис не поднимается, логика живёт в пакете `publishing/`.

## Зачем это нужно

- **Черновик** (storage) — для редактирования, JSON-мутации в MongoDB.
- **Публикация** — неизменяемый снимок для просмотра: статические файлы в object storage.

Создание публикации **асинхронное**: HTTP-ручка быстро создаёт запись со статусом `PENDING` и ставит задачу в RabbitMQ; worker в том же процессе выполняет рендер и загрузку в S3, обновляя статус до `FINISHED` или `FAILED`. Клиент опрашивает `GET …/publications/{id}` до завершения.

Сборка выполняется через [landing-builder-cli v2](https://github.com/rki-mai/landing-builder-cli) (Go + Astro): в bundle попадают `index.html` и сгенерированные ассеты (CSS, JS и др.). Интерфейс `BlobStorage` загружает все файлы из output-директории CLI.

## Структура пакета

```
publishing/
├── handler.go      # HTTP API
├── models.go       # Publication, статусы, DTO
├── repository.go   # MongoDB: коллекция publications
├── service.go      # Create (enqueue) / ProcessPublication (worker)
├── worker/         # consumer RabbitMQ
├── README.md
└── utils/
    ├── blobstorage.go    # интерфейс BlobStorage
    ├── s3_blobstorage.go # реализация S3 / MinIO
    ├── renderer.go       # рендер JSON → bundle (landing-builder-cli build)
    ├── draft.go          # снимок Draft (парсинг BSON/JSON)
    ├── draft_reader.go   # чтение черновика из storage
    ├── queue.go          # интерфейсы Publisher/Consumer
    └── rabbitmq.go       # реализация RabbitMQ
```

По аналогии с `auth/` и `storage/`: в корне — слой API и домена, в `utils/` — инфраструктурные детали и адаптеры.

## API

Все ручки требуют JWT (`Authorization: Bearer <token>`).

| Метод | Путь | Описание |
|-------|------|----------|
| `GET` | `/api/v1/storage/{project_id}/publications` | Список ID публикаций проекта |
| `POST` | `/api/v1/storage/{project_id}/publications` | Поставить публикацию в очередь (ответ `status: PENDING`) |
| `GET` | `/api/v1/storage/{project_id}/publications/{id}` | Получить метаданные и статус (`PENDING` / `PROCESSING` / `FINISHED` / `FAILED`) |
| `DELETE` | `/api/v1/storage/{project_id}/publications/{id}` | Удалить публикацию и bundle в S3 |

Документация в Swagger: http://localhost:8080/swagger/index.html (тег **Publications**).

### Публичный просмотр (CDN)

После публикации (`status: FINISHED`) отрендеренный лендинг доступен по публичной ссылке **без JWT**:

| Метод | Путь | Описание |
|-------|------|----------|
| `GET` | `/publications/{publication_id}/index.html` | HTML-страница лендинга (через nginx CDN) |
| `GET` | `/publications/{publication_id}/{path...}` | Любой файл из bundle (когда появятся CSS/JS/медиа) |

Пример: `http://localhost:8080/publications/550e8400-e29b-41d4-a716-446655440000/index.html`

Поток доставки: **MinIO (S3)** → **nginx CDN** (`proxy_cache`) → браузер. API-ручки `GET/POST …/publications` возвращают поле `public_url` с готовой ссылкой.

Единая точка входа на `:8080` — сервис `cdn` в `docker-compose.yml`:

| Путь | Назначение |
|------|------------|
| `/api/*` | backend API |
| `/swagger/*` | Swagger UI |
| `/publications/*` | MinIO (опубликованные лендинги, с кэшем) |
| `/*` | статические файлы UI из `docker/static/` (nginx `autoindex`) |

Bucket `publications` настроен на anonymous download (только GET объектов).

## Быстрый старт

### Makefile (рекомендуется)

Команды выполнять из каталога с `main.go` и `docker-compose.yml` (`wb-landing-builder/wb-landing-builder/`):

```bash
make help          # список целей
make up            # поднять MongoDB, MinIO и API
make swag          # перегенерировать Swagger (локально, нужен Go)
make swag-docker   # то же через Docker, без Go на хосте
make rebuild       # swag + пересборка контейнера API
make test-unit     # unit-тесты Go (без Docker)
make test-smoke    # сквозной HTTP-тест (curl): auth → storage → publishing
make test          # go build + unit + smoke
```

Переменные окружения для smoke-теста:

| Переменная | По умолчанию | Назначение |
|------------|--------------|------------|
| `BASE_URL` | `http://localhost:8080` | URL API |
| `PROJECT_ID` | `smoke-<timestamp>` | ID проекта в тесте |
| `SMOKE_EMAIL` | `smoke-<timestamp>@example.com` | email тестового пользователя |
| `SMOKE_VERBOSE` | `0` | `1` — печатать полный JSON ответов через jq |
| `SMOKE_SKIP_CLEANUP` | `0` | `1` — не удалять тестовые данные из MongoDB |
| `SMOKE_MONGO_CONTAINER` | `landing-mongo` | контейнер MongoDB для cleanup |

После успешного прогона (и при падении теста) скрипт удаляет **только данные, созданные в этом запуске**:

- мутации и snapshots черновика — `project_id` текущего прогона **и** `owner_id` зарегистрированных в тесте пользователей;
- публикации — только по `_id`, полученным в этом прогоне (не все публикации проекта);
- пользователей и refresh-токены — только по `_id`/`user_id` из ответов `register`.

Чужие проекты, пользователи и публикации не затрагиваются.

Пример:

```bash
make up
make test-smoke
# полный JSON на каждом шаге:
SMOKE_VERBOSE=1 make test-smoke
```

Скрипт `scripts/smoke.sh` регистрирует пользователя, собирает черновик из `scripts/fixtures/mutations/`, создаёт публикацию, проверяет list/get, проверяет 403 для чужого пользователя, удаляет публикацию через API и **очищает MongoDB** (черновик + пользователи). На каждом шаге выводит краткую сводку; при ошибке — отформатированный JSON через jq.

### 1. Поднять окружение вручную

Из каталога с `main.go` и `docker-compose.yml`:

```bash
docker compose up -d --build
```

Поднимаются MongoDB, MinIO, бэкенд. Проверка:

```bash
docker compose ps
```

### 2. Обновить Swagger (если меняли аннотации в handler)

Команды выполнять из каталога `wb-landing-builder/wb-landing-builder` (рядом с `main.go`), либо через `make swag` / `make swag-docker` из каталога compose.

**Локально** (нужны Go и сеть для `go install`):

```bash
make swag
make up
```

или вручную:

```bash
go mod download
go install github.com/swaggo/swag/cmd/swag@v1.16.6
$(go env GOPATH)/bin/swag init -g main.go -o docs --parseDependency --parseInternal
docker compose up -d --build wb-landing-builder
```

**Через Docker** (Go на хосте не нужен):

```bash
make swag-docker
make up
```

### 3. Проверка через Swagger UI

1. Открыть http://localhost:8080/swagger/index.html
2. **Auth** → `POST /api/v1/auth/register` и `POST /api/v1/auth/login`
3. **Authorize** → `Bearer <access_token>`
4. **Storage** — собрать черновик для `demo-project` (см. ниже)
5. **Publications** → `POST /api/v1/storage/demo-project/publications`
6. В MinIO Console (http://localhost:9001, `minioadmin` / `minioadmin`) в bucket `publications` должны появиться `index.html` и связанные ассеты

#### Черновик в Storage (как `storage-sample.json`)

Целевой снимок — [landing-builder-cli/examples/storage-sample.json](../../../landing-builder-cli/examples/storage-sample.json): контейнер, текст, вложенный блок с кнопкой и ссылка.

В Swagger: **Storage** → `POST /api/v1/storage/{project_id}/mutations`, `project_id` = `demo-project`.  
Отправить **шесть запросов `create` по порядку** (дочерние элементы — только после родителя):

**1. Контейнер**

```json
{
  "operation": "create",
  "data": {
    "element": "container",
    "id": "lb-1",
    "parentId": "root",
    "index": 0,
    "styles": {
      "display": "flex",
      "flexDirection": "column",
      "padding": "20px"
    }
  }
}
```

**2. Заголовок**

```json
{
  "operation": "create",
  "data": {
    "element": "text",
    "id": "lb-2",
    "parentId": "lb-1",
    "index": 0,
    "styles": {
      "color": "#333333",
      "fontSize": "24px"
    },
    "value": "Привет, Мир!"
  }
}
```

**3. Вложенный контейнер**

```json
{
  "operation": "create",
  "data": {
    "element": "container",
    "id": "lb-3",
    "parentId": "lb-1",
    "index": 1,
    "styles": {
      "display": "flex",
      "gap": "10px",
      "marginTop": "20px"
    }
  }
}
```

**4. Текст во вложенном блоке**

```json
{
  "operation": "create",
  "data": {
    "element": "text",
    "id": "lb-4",
    "parentId": "lb-3",
    "index": 0,
    "styles": {
      "color": "#666666",
      "fontSize": "16px"
    },
    "value": "Это пример вложенной структуры."
  }
}
```

**5. Кнопка**

```json
{
  "operation": "create",
  "data": {
    "element": "button",
    "id": "lb-5",
    "parentId": "lb-3",
    "index": 1,
    "styles": {
      "backgroundColor": "#007bff",
      "color": "#ffffff",
      "padding": "10px"
    },
    "value": "Нажми меня",
    "src": "https://example.com"
  }
}
```

**6. Ссылка**

```json
{
  "operation": "create",
  "data": {
    "element": "link",
    "id": "lb-6",
    "parentId": "lb-1",
    "index": 2,
    "styles": {
      "color": "blue",
      "textDecoration": "underline"
    },
    "value": "Подробнее",
    "src": "https://example.com"
  }
}
```

Проверка черновика: `GET /api/v1/storage/demo-project` — в ответе массив из шести элементов (как в sample). Затем шаг 5 с публикацией.

### Переменные окружения (S3 / CLI / RabbitMQ)

| Переменная | Назначение | По умолчанию в compose |
|------------|------------|-------------------------|
| `S3_ENDPOINT` | URL MinIO / S3 | `http://minio:9000` |
| `S3_BUCKET` | Имя bucket | `publications` |
| `S3_ACCESS_KEY` / `S3_SECRET_KEY` | Учётные данные | `minioadmin` |
| `S3_USE_PATH_STYLE` | Path-style для MinIO | `true` |
| `PUBLISHING_CLI_PATH` | Путь к бинарнику `landing-builder-cli` | `/app/cli/landing-builder-cli` |
| `RABBITMQ_URL` | URL брокера | `amqp://guest:guest@rabbitmq:5672/` |
| `RABBITMQ_PUBLISH_QUEUE` | Очередь задач на рендер | `publish.requests` |
| `PUBLIC_BASE_URL` | Базовый URL для `public_url` в ответах API | `http://localhost:8080` |

## Тестирование

### Unit-тесты (publishing)

Без Docker и внешних сервисов — проверяют async-логику `PublicationService` на моках (Mongo, S3, renderer, RabbitMQ):

```bash
make test-unit
# только publishing:
go test ./publishing/...
```

Покрывают: `Create` → `PENDING` без синхронного рендера, `ProcessPublication` → `FINISHED`, ошибку постановки в очередь, удаление.

### Автоматический smoke-тест (рекомендуется)

Из каталога с `main.go` и `docker-compose.yml`:

```bash
make up
make test-smoke
```

Скрипт `scripts/smoke.sh` прогоняет полный сценарий без Swagger: регистрация → login → 6 mutations → GET draft → POST publication → ожидание FINISHED → GET HTML через CDN → list/get → проверка 403 для чужого пользователя → DELETE.

Фикстуры мутаций: `scripts/fixtures/mutations/` (тот же набор, что в разделе «Черновик в Storage» ниже).

### Нагрузочное тестирование storage

В `storage/test.py` — нагрузочные проверки mutations API (aiohttp), не путать со smoke.

### Ручная проверка

Swagger UI + MinIO Console (http://localhost:9001) — см. раздел «Проверка через Swagger UI».

## Связанные материалы

- ADR: [Сервис публикации лендингов](../../../docs/ADRs/Сервис%20публикации%20лендингов.md)
- CLI рендера: [landing-builder-cli](https://github.com/rki-mai/landing-builder-cli)
- Общий запуск проекта: [CONTRIBUTING.md](../CONTRIBUTING.md)
