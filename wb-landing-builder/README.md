Вот обновлённый `README.md`, дополненный:

- ✅ **проверкой запуска MongoDB**  
- ✅ **полную таблицу всех эндпоинтов**  
- ✅ **пошаговый сквозной пример**, использующий **все ручки**, с реальными ссылками и «глубоким» контентом (включая мем 🐸)  
- ✅ **советы по отладке и проверке состояния**  
- ✅ улучшенную структуру и читаемость

---

# 🛠️ Конструктор лендингов — Backend MVP

Внутренний сервис для создания, редактирования и публикации лендингов без участия разработчиков.  
**Backend реализован на Go (Gin + MongoDB)**.

---

## 📦 Требования

- [Go](https://golang.org/) 1.20+
- [Docker](https://www.docker.com/) (для запуска MongoDB)
- `curl`, `jq`, и `httpie` (опционально) — для удобной работы с API  
  *(например: [HTTPie](https://httpie.io/))*


---

## 🚀 Быстрый старт

### 1. Запустите MongoDB

```bash
# Удалить старый контейнер (если был)
docker rm -f mongo-dev 2>/dev/null

# Запустить новый
docker run -d -p 27017:27017 --name mongo-dev mongo:latest --bind_ip_all
```

#### ✅ Как проверить, что MongoDB запущена и доступна?

```bash
# Проверить статус контейнера:
docker ps --filter name=mongo-dev --format "table {{.Status}}\t{{.Names}}"

# Проверить подключение (без внешних утилит):
docker exec -it mongo-dev mongosh --eval "db.runCommand({ping:1})" landing_builder
```

Ожидаемый вывод:  
```
{ ok: 1 }
```

> Если `Connection refused` или контейнер не запущен — перезапустите его.  
> Сервер Go **не стартует**, если MongoDB недоступна (падает с `log.Fatal`).

---

### 2. Запустите сервер

```bash
go run main.go
```

Успешный старт:
```
✅ Подключились к MongoDB
[GIN-debug] Listening and serving HTTP on :8080
```

> 🚨 Если видите `context deadline exceeded` — MongoDB недоступна. Проверьте шаг выше.

---

## 📡 Полный список API-эндпоинтов

| Метод | Путь                          | Описание                                       | Требует `published`? |
|-------|-------------------------------|------------------------------------------------|----------------------|
| `POST`   | `/landings`                   | Создать **новый черновик**                     | —                    |
| `GET`    | `/landings/:id`               | Получить лендинг по ID (внутренний просмотр)  | —                    |
| `PUT`    | `/landings/:id`               | Обновить черновик (нельзя опубликовать через PUT!) | —                |
| `DELETE` | `/landings/:id`               | Удалить **только неопубликованный** лендинг   | —                    |
| `POST`   | `/landings/:id/publish`       | Опубликовать лендинг                          | —                    |
| `POST`   | `/landings/:id/unpublish`     | Снять с публикации                            | —                    |
| `GET`    | `/public/:id`                 | Публичная HTML-страница лендинга              | ✅ Только если `published: true` |
| `GET`    | `/landings/table`             | HTML-таблица всех лендингов (админка)         | —                    |

> 🔐 **Важно**:  
> - Через `PUT /landings/:id` нельзя изменить `published` или `publicUrl` — они наследуются от существующего документа.  
> - Удаление запрещено для опубликованных лендингов — сначала нужно `/unpublish`.

---

## 🧪 Сквозной пример: от нуля до публикации

Давайте создадим полноценный лендинг «День программиста» с мемом, заголовками, кнопками и разделителями.

### 🔹 Шаг 1: Создаём лендинг

```bash
ID=$(curl -s -X POST http://localhost:8080/landings \
  -H "Content-Type: application/json" \
  -d '{
    "title": "День программиста 2025",
    "blocks": [
      {
        "type": "hero_banner",
        "props": {
          "title": "13-е сентября — День программиста!",
          "imageUrl": "https://steamuserimages-a.akamaihd.net/ugc/1857173701239457542/B2DD80C8CF0457AE74E41460D2A28E5603B3B9D6/?imw=800&imh=400&ima=fit&impolicy=Letterbox&imcolor=%23000000&letterbox=true"
        }
      },
      {
        "type": "paragraph",
        "props": {
          "text": "С праздником, коллеги! Сегодня — 256-й день года (в високосный — 255-й), и он посвящён тем, кто говорит на языке машин и понимает их лучше людей."
        }
      },
      {
        "type": "heading",
        "props": {
          "level": 2,
          "text": "Почему именно 256?"
        }
      },
      {
        "type": "paragraph",
        "props": {
          "text": "2⁸ = 256 — число, умещающееся в один байт, и максимальное количество значений, которое можно закодировать одним байтом."
        }
      },
      {
        "type": "image",
        "props": {
          "src": "https://steamuserimages-a.akamaihd.net/ugc/1857173701239457542/B2DD80C8CF0457AE74E41460D2A28E5603B3B9D6/?imw=512&imh=329&ima=fit&impolicy=Letterbox&imcolor=%23000000&letterbox=true",
          "alt": "Мем: Программест, не программист",
          "width": "70%"
        }
      },
      {
        "type": "spacer",
        "props": {
          "height": "30px"
        }
      },
      {
        "type": "button",
        "props": {
          "text": "Поделиться в Slack",
          "url": "https://wildberries.slack.com/channels/team-dev",
          "variant": "primary"
        }
      },
      {
        "type": "divider"
      },
      {
        "type": "html",
        "props": {
          "html": "<blockquote style=\"font-style:italic; border-left:4px solid #007bff; padding-left:16px; color:#555;\">«Если бы в школе учили программированию, я бы стал отличником»</blockquote>"
        }
      }
    ]
  }' | jq -r '.id')

echo "✅ Лендинг создан. ID: $ID"
```

> 📌 Сохранён в переменную `$ID` для дальнейшего использования.

---

### 🔹 Шаг 2: Посмотрим черновик (внутренний просмотр)

```bash
curl -s "http://localhost:8080/landings/$ID" | jq .
```

Проверьте:
- `published: false`
- `publicUrl: ""`
- блоки — как в запросе

---

### 🔹 Шаг 3: Отредактируем (добавим подзаголовок в `hero_banner`)

```bash
curl -s -X PUT "http://localhost:8080/landings/$ID" \
  -H "Content-Type: application/json" \
  -d "{
    \"title\": \"День программиста 2025\",
    \"blocks\": [
      {
        \"type\": \"hero_banner\",
        \"props\": {
          \"title\": \"13-е сентября\",
          \"subtitle\": \"День программиста!\",
          \"imageUrl\": \"https://steamuserimages-a.akamaihd.net/ugc/1857173701239457542/B2DD80C8CF0457AE74E41460D2A28E5603B3B9D6/?imw=800&imh=400&ima=fit&impolicy=Letterbox&imcolor=%23000000&letterbox=true\"
        }
      },
      {
        \"type\": \"paragraph\",
        \"props\": {
          \"text\": \"С праздником, коллеги! Сегодня — 256-й день года...\"
        }
      }
    ]
  }"
```

> ⚠️ `PUT` **перезаписывает весь `blocks` массив** — передайте полную структуру!  
> (В будущем можно реализовать частичное обновление — PATCH.)

---

### 🔹 Шаг 4: Опубликуем

```bash
curl -s -X POST "http://localhost:8080/landings/$ID/publish" | jq .
```

Убедитесь, что в ответе:
- `"published": true`
- `"publicUrl": "/public/...$ID"`

---

### 🔹 Шаг 5: Откроем публичную страницу

В браузере откройте:

```
http://localhost:8080/public/$ID
```

👉 Вы увидите красиво оформленный лендинг с:
- героическим баннером и мемом «Программест» 🐸  
- пояснением про 256  
- кнопкой и цитатой в HTML-блоке (не экранированной!)

---

### 🔹 Шаг 6: Снимем с публикации (если нужно)

```bash
curl -s -X POST "http://localhost:8080/landings/$ID/unpublish" | jq .
```

Теперь `/public/$ID` вернёт `404`.

---

### 🔹 Шаг 7: Посмотрим все лендинги

Откройте в браузере:  
👉 [http://localhost:8080/landings/table](http://localhost:8080/landings/table)

Вы увидите таблицу с:
- вашим лендингом
- кнопкой «Опубликовать» / «Убрать из публикации»
- ссылками на просмотр

> 💡 Таблица использует **DataTables** — поддерживаются сортировка, поиск и пагинация.

---

### 🔹 Шаг 8: Удалим (если не опубликован)

```bash
# Сначала убедимся, что не опубликован:
curl -s "http://localhost:8080/landings/$ID" | jq '.published'

# Если false — удаляем:
curl -X DELETE "http://localhost:8080/landings/$ID" -w "\nStatus: %{http_code}\n"
# Ожидаем: Status: 204
```

---

## 🧱 Поддерживаемые блоки (на 2025-12-10)

| Тип (`type`)        | Описание                        | Пример `props` |
|---------------------|----------------------------------|----------------|
| `heading`           | Заголовок `<h1>`–`<h6>`          | `{"level":2, "text":"Тайтл"}` |
| `paragraph`         | Абзац `<p>`                      | `{"text":"..."}` |
| `image`             | Изображение `<img>`              | `{"src":"...", "alt":"...", "width":"100%"}` |
| `button`            | Кнопка-ссылка                    | `{"text":"Клик", "url":"https://..."}` |
| `html`              | **Raw HTML** (опасно!)           | `{"html":"<b>Жирный</b>"}` |
| `divider`           | Горизонтальная линия `<hr>`      | `{}` |
| `spacer`            | Отступ `<div style="height:...">`| `{"height":"40px"}` |
| `hero_banner`       | Баннер с заголовком + изображением| `{"title":"...", "imageUrl":"..."}` |

> 🧠 Подсказка: если передать неизвестный `type`, сервер выведет warning в лог и отобразит заглушку в HTML.

---

## 📁 Структура проекта

```
.
├── main.go                 # Точка входа, маршруты Gin
├── db/
│   └── db.go               # Подключение к MongoDB
├── models/
│   └── landing.go          # Структуры: Landing, Block
└── repository/
    └── landing_repo.go     # CRUD: Create, GetByID, Update, Delete, List
```

---

## 🛑 Остановка

```bash
# Сервер: Ctrl+C

# MongoDB:
docker stop mongo-dev
# (или docker rm -f mongo-dev — остановит + удалит)
```

---

## 📝 Примечания

- Все ID — **24-символьные hex-строки** (ObjectID MongoDB), например: `675a1b2c3d4e5f6a7b8c9d0e`
- Не используйте числовые ID (`1`, `2`) — они не валидны для `primitive.ObjectIDFromHex`
- `curl -w "\n"` помогает избежать «слипания» вывода с приглашением терминала
- Для удобства используйте `jq` для форматирования JSON:
  ```bash
  curl ... | jq .
  ```

---

## 🧩 Дальнейшее развитие (Roadmap)

| Приоритет | Фича                              | Комментарий |
|-----------|-----------------------------------|-------------|
| ⭐ High    | Валидация структуры блоков         | JSON Schema или custom validators |
| ⭐ High    | Авторизация (JWT / SSO)            | Защита `/landings/*`, кроме `/public/*` |
| ⭐ Medium  | Частичное обновление — `PATCH /landings/:id` | Не перезаписывать весь объект |
| ⭐ Medium  | Генерация статики (`/static/:id/index.html`) | Выгрузка в S3 / CDN |
| ⭐ Low     | Версионирование лендингов         | Draft history, rollback |
| ⭐ Low     | Preview-режим (`/preview/:id`)    | Просмотр черновика без публикации |

---

> 🎯 **Цель MVP**: дать маркетологам и продюсерам возможность быстро собирать и публиковать простые промо-страницы **без участия разработчиков**.

