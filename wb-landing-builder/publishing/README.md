# Компонент публикаций (publishing)

Компонент отвечает за публикацию черновиков лендингов: берёт актуальный снимок из **storage**, собирает bundle файлов и сохраняет его в S3-совместимое хранилище (локально — MinIO). Метаданные публикации хранятся в MongoDB.

В монолите это реализация идеи **Publishing Service** из ADR «Сервис публикации лендингов» — отдельный микросервис не поднимается, логика живёт в пакете `publishing/`.

## Зачем это нужно

- **Черновик** (storage) — для редактирования, JSON-мутации в MongoDB.
- **Публикация** — неизменяемый снимок для просмотра: статические файлы в object storage.

Сейчас сборка упрощена (задача #18): в bundle кладутся `index.json` (исходный snapshot) и `index.html` (рендер через [landing-builder-cli](https://github.com/rki-mai/landing-builder-cli)). Полноценная сборка CSS/JS/медиа — в отдельной задаче; интерфейс `BlobStorage` уже рассчитан на несколько файлов в bundle.

## Структура пакета

```
publishing/
├── handler.go      # HTTP API
├── models.go       # Publication, запросы, ошибки
├── repository.go   # MongoDB: коллекция publications
├── service.go      # сценарий Create / Get / Delete
├── README.md
└── utils/
    ├── blobstorage.go    # интерфейс BlobStorage
    ├── s3_blobstorage.go # реализация S3 / MinIO
    ├── renderer.go       # рендер JSON → HTML (CLI)
    ├── draft_reader.go   # чтение черновика из storage
    └── httputil.go       # JSON-ответы
```

По аналогии с `auth/` и `storage/`: в корне — слой API и домена, в `utils/` — инфраструктурные детали и адаптеры.

## API

Все ручки требуют JWT (`Authorization: Bearer <token>`).

| Метод | Путь | Описание |
|-------|------|----------|
| `POST` | `/api/v1/publications` | Создать публикацию по `project_id` |
| `GET` | `/api/v1/publications/{id}` | Получить метаданные |
| `DELETE` | `/api/v1/publications/{id}` | Удалить публикацию и bundle в S3 |

Документация в Swagger: http://localhost:8080/swagger/index.html (тег **Publications**).

## Быстрый старт

### 1. Поднять окружение

Из каталога с `docker-compose.yml`:

```bash
docker compose up -d --build
```

Поднимаются MongoDB, MinIO, бэкенд. Проверка:

```bash
docker compose ps
```

### 2. Обновить Swagger (если меняли аннотации в handler)

Команды выполнять из каталога `wb-landing-builder/wb-landing-builder` (рядом с `main.go`).

**Локально** (нужны Go и сеть для `go install`):

```bash
go mod download
go install github.com/swaggo/swag/cmd/swag@v1.16.6
swag init -g main.go -o docs --parseDependency --parseInternal
docker compose up -d --build wb-landing-builder
```

**Через Docker** (Go на хосте не нужен; `docs/` перезапишется в текущей директории):

```bash
docker run --rm \
  -v "$(pwd):/app" \
  -w /app \
  golang:1.25.10-alpine \
  sh -c 'apk add --no-cache git && go install github.com/swaggo/swag/cmd/swag@v1.16.6 && /root/go/bin/swag init -g main.go -o docs --parseDependency --parseInternal'

docker compose up -d --build wb-landing-builder
```

### 3. Проверка через Swagger UI

1. Открыть http://localhost:8080/swagger/index.html
2. **Auth** → `POST /api/v1/auth/register` и `POST /api/v1/auth/login`
3. **Authorize** → `Bearer <access_token>`
4. **Storage** — собрать черновик для `demo-project` (см. ниже)
5. **Publications** → `POST /api/v1/publications` с телом `{ "project_id": "demo-project" }`
6. В MinIO Console (http://localhost:9001, `minioadmin` / `minioadmin`) в bucket `publications` должны появиться `index.json` и `index.html`

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

### Переменные окружения (S3 / CLI)

| Переменная | Назначение | По умолчанию в compose |
|------------|------------|-------------------------|
| `S3_ENDPOINT` | URL MinIO / S3 | `http://minio:9000` |
| `S3_BUCKET` | Имя bucket | `publications` |
| `S3_ACCESS_KEY` / `S3_SECRET_KEY` | Учётные данные | `minioadmin` |
| `S3_USE_PATH_STYLE` | Path-style для MinIO | `true` |
| `PUBLISHING_CLI_PATH` | Путь к `generate.py` | `/app/cli/generate.py` |

## Тестирование

В других компонентах интеграционные проверки делаются через HTTP-скрипты (например, `storage/test.py`), а не через `*_test.go` в Go. Для **publishing** достаточно ручной проверки по Swagger и просмотра объектов в MinIO; отдельный `service_test.go` в репозитории не используется.

При необходимости позже можно добавить `publishing/test.py` по образцу storage.

## Связанные материалы

- ADR: [Сервис публикации лендингов](../../../docs/ADRs/Сервис%20публикации%20лендингов.md)
- CLI рендера: [landing-builder-cli](https://github.com/rki-mai/landing-builder-cli)
- Общий запуск проекта: [CONTRIBUTING.md](../CONTRIBUTING.md)
