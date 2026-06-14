# Quick Start — ручная проверка

Краткая инструкция: поднять стек и убедиться, что auth, draft storage, revert и publishing работают.

## Что нужно

- **Docker** (Compose v2 или `docker-compose`)
- **Браузер** — для ручной проверки через Swagger
- **curl**, **jq** — только для автоматического smoke (`make test-smoke`)
- **Go 1.20+** — только для unit-тестов без Docker (`make test-unit`)

Все команды ниже — из каталога `wb-landing-builder/` (где `main.go` и `docker-compose.yml`).

## 1. Поднять стек

```bash
make up
make ps   # дождаться healthy (обычно 30–60 с)
```

API и CDN: **http://localhost:8080**

## 2. Быстрая проверка «жив ли API»

Откройте в браузере:

- http://localhost:8080/swagger/index.html — Swagger UI (должен загрузиться без ошибок)
- http://localhost:8080/index.html — статический UI редактора

## 3. Автоматическая проверка (рекомендуется)

```bash
make test-unit   # go test, Docker не нужен
make test        # unit + smoke (нужен make up)
```

Smoke проходит полный сценарий: auth → 6 create-мутаций → update → revert → delete → revert → публикация → CDN.

Только smoke:

```bash
make test-smoke
SMOKE_VERBOSE=1 make test-smoke   # подробный вывод
```

## 4. Ручная проверка через Swagger

Swagger: **http://localhost:8080/swagger/index.html**

В Auth уже подставлены примеры (`newuser@example.com` / `SuperSecretPass123`). Для мутаций готовых примеров нет — JSON ниже копируйте в тело запроса **Storage → POST /api/v1/projects/{project_id}/draft/mutations**.

### 4.1. Регистрация и вход

1. **Auth → POST /api/v1/auth/register** — *Try it out* → *Execute*  
   При `409 Conflict` смените email в теле запроса (должен быть уникальным).
2. **Auth → POST /api/v1/auth/login** — *Execute*, скопируйте `access_token` из ответа.
3. Нажмите **Authorize** (вверху справа), введите:
   ```
   Bearer <access_token>
   ```
   (*Authorize* → *Close*).

### 4.2. Создать проект

**Storage → POST /api/v1/projects** — тело `{}` → *Execute*.

Скопируйте `project_id` — он понадобится во всех следующих запросах (`project_id` в path).

### 4.3. Наполнить черновик (create)

Для каждой мутации: **Storage → POST …/draft/mutations** → вставить JSON → *Execute*.

Порядок важен — как в smoke-фикстурах:

<details>
<summary>1. Контейнер (lb-1)</summary>

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

</details>

<details>
<summary>2. Заголовок (lb-2) — «Привет, Мир!»</summary>

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

</details>

<details>
<summary>3–6. Остальные элементы (опционально, для полного smoke-сценария)</summary>

Полный набор — файлы `../scripts/fixtures/mutations/03-*.json` … `06-*.json`.  
Для проверки revert достаточно шагов 1–2.

</details>

Проверка: **Storage → GET /api/v1/projects/{project_id}/draft** — в `elements` есть `lb-2` со значением `Привет, Мир!`.

### 4.4. Update → revert (основная проверка)

**Update** — изменить заголовок:

```json
{
  "operation": "update",
  "data": {
    "id": "lb-2",
    "fields": {
      "value": "Updated heading"
    }
  }
}
```

*GET …/draft* → `lb-2.value` = `Updated heading`.

**Revert(1)** — откат последней мутации:

```json
{
  "operation": "revert",
  "data": {
    "count": 1
  }
}
```

*GET …/draft* → снова `Привет, Мир!`.

### 4.5. Delete → revert (опционально)

Сначала добавьте кнопку `lb-5` (фикстура `05-button.json`) или пропустите, если делали только lb-1/lb-2.

**Delete** — удалить кнопку:

```json
{
  "operation": "delete",
  "data": {
    "id": "lb-5"
  }
}
```

*GET …/draft* → элементов на 1 меньше, `lb-5` нет.

**Revert(1)** — вернуть удалённое:

```json
{
  "operation": "revert",
  "data": {
    "count": 1
  }
}
```

*GET …/draft* → `lb-5` снова на месте.

### 4.6. Публикация (опционально)

1. **Publications → POST /api/v1/projects/{project_id}/publications** — `{}`.
2. **GET …/publications/{publication_id}** — дождаться `"status": "FINISHED"`.
3. Открыть `public_url` из ответа в браузере — HTML лендинга.

---

Готовые JSON для всех мутаций: `../scripts/fixtures/mutations/`.

## 5. Полезные URL

| URL | Назначение |
|-----|------------|
| http://localhost:8080/swagger/index.html | Swagger UI |
| http://localhost:8080/index.html | Статический UI редактора |
| http://localhost:8081 | mongo-express (Mongo UI) |
| http://localhost:9001 | MinIO Console (`minioadmin` / `minioadmin`) |
| http://localhost:15672 | RabbitMQ Management (`guest` / `guest`) |

## 6. Если что-то пошло не так

**Smoke падает на static 404** — перезапустить CDN:

```bash
docker compose restart cdn
```

**401 в Swagger** — заново *Login* → *Authorize* с новым токеном (истекает).

**400 на мутации** — проверьте JSON и порядок create (родитель должен существовать раньше дочернего).

**API не отвечает:**

```bash
make logs
make rebuild   # после изменений в коде
make down      # остановить стек
```

## Шпаргалка make-целей

```bash
make help        # все цели
make up          # поднять стек
make test-unit   # go test ./...
make test-smoke  # HTTP smoke (нужен make up)
make test        # unit + smoke
make down        # остановить
```
