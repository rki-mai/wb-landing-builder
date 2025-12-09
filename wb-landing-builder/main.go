// main.go
package main

import (
	"context"
	"fmt"
	"html"
	"log"
	"strings"
	"wb-landing-builder/db"
	"wb-landing-builder/models"
	"wb-landing-builder/repository"

	"github.com/gin-gonic/gin"
)

func main() {
	db.Init()
	r := gin.Default()

	// Создать лендинг
	r.POST("/landings", func(c *gin.Context) {
		var landing models.Landing
		if err := c.ShouldBindJSON(&landing); err != nil {
			c.JSON(400, gin.H{"error": err.Error()})
			return
		}
		landing.Published = false
		insertedID, err := repository.CreateLanding(&landing)
		if err != nil {
			c.JSON(500, gin.H{"error": "Failed to save"})
			return
		}
		// Обновляем ID в структуре перед отправкой ответа
		landing.ID = insertedID
		c.JSON(201, landing)
	})

	// Получить лендинг
	r.GET("/landings/:id", func(c *gin.Context) {
		id := c.Param("id")
		landing, err := repository.GetLandingByID(id)
		if err != nil {
			c.JSON(404, gin.H{"error": "Not found"})
			return
		}
		c.JSON(200, landing)
	})

	// Опубликовать (упрощённо: просто включаем флаг)
	r.POST("/landings/:id/publish", func(c *gin.Context) {
		id := c.Param("id")
		landing, err := repository.GetLandingByID(id)
		if err != nil {
			c.JSON(404, gin.H{"error": "Not found"})
			return
		}
		landing.Published = true
		landing.PublicUrl = "/public/" + id // или генерировать короткий ID
		if err := repository.UpdateLanding(id, landing); err != nil {
			c.JSON(500, gin.H{"error": "Publish failed"})
			return
		}
		c.JSON(200, landing)
	})

	// Снять с публикации
	r.POST("/landings/:id/unpublish", func(c *gin.Context) {
		id := c.Param("id")
		landing, err := repository.GetLandingByID(id)
		if err != nil {
			c.JSON(404, gin.H{"error": "Not found"})
			return
		}
		landing.Published = false
		landing.PublicUrl = "" // очищаем
		if err := repository.UpdateLanding(id, landing); err != nil {
			c.JSON(500, gin.H{"error": "Unpublish failed"})
			return
		}
		c.JSON(200, landing)
	})

	// Публичный просмотр — полноценный рендер лендинга на основе блоков
	r.GET("/public/:id", func(c *gin.Context) {
		id := c.Param("id")
		landing, err := repository.GetLandingByID(id)
		if err != nil || !landing.Published {
			c.Status(404)
			return
		}

		var htmlContent strings.Builder

		for _, block := range landing.Blocks {
			props := block.Props

			switch block.Type {
			case "heading":
				// Ожидаем: {"level": 1, "text": "Заголовок"}
				if text, ok := props["text"].(string); ok {
					level := 1
					if l, ok := props["level"].(float64); ok {
						level = int(l)
						if level < 1 {
							level = 1
						} else if level > 6 {
							level = 6
						}
					}
					htmlContent.WriteString(fmt.Sprintf("<h%d>%s</h%d>", level, html.EscapeString(text), level))
				}

			case "paragraph":
				// Ожидаем: {"text": "Абзац текста"}
				if text, ok := props["text"].(string); ok {
					htmlContent.WriteString(fmt.Sprintf("<p>%s</p>", html.EscapeString(text)))
				}

			case "image":
				// Ожидаем: {"src": "https://...", "alt": "...", "width": "100%"}
				if src, ok := props["src"].(string); ok {
					alt := ""
					if a, ok := props["alt"].(string); ok {
						alt = a
					}
					width := "100%"
					if w, ok := props["width"].(string); ok {
						width = w
					}
					htmlContent.WriteString(fmt.Sprintf(
						`<img src="%s" alt="%s" width="%s" style="height:auto; display:block;">`,
						html.EscapeString(src),
						html.EscapeString(alt),
						html.EscapeString(width),
					))
				}

			case "button":
				// Ожидаем: {"text": "Клик", "url": "https://...", "variant": "primary"}
				if text, ok := props["text"].(string); ok {
					url := "#"
					if u, ok := props["url"].(string); ok {
						url = u
					}
					htmlContent.WriteString(fmt.Sprintf(
						`<a href="%s" target="_blank" style="display:inline-block; padding:10px 20px; background:#007bff; color:white; text-decoration:none; border-radius:4px; margin:8px 0;">%s</a>`,
						html.EscapeString(url),
						html.EscapeString(text),
					))
				}

			case "html":
				// ⚠️ Опасно! Только если доверяете автору.
				if raw, ok := props["html"].(string); ok {
					// НЕ экранируем — разрешаем raw HTML
					htmlContent.WriteString(raw)
				}

			case "divider":
				htmlContent.WriteString("<hr style='border:0; border-top:1px solid #eee; margin:24px 0;'>")

			case "spacer":
				height := "20px"
				if h, ok := props["height"].(string); ok {
					height = h
				}
				htmlContent.WriteString(fmt.Sprintf(`<div style="height:%s;"></div>`, html.EscapeString(height)))

			case "hero_banner":
				// Ожидаем: {"title": "Заголовок", "imageUrl": "https://..."}
				title := ""
				if t, ok := props["title"].(string); ok {
					title = t
				}
				imageUrl := ""
				if u, ok := props["imageUrl"].(string); ok {
					imageUrl = strings.TrimSpace(u) // убираем случайные пробелы из curl
				}

				if imageUrl != "" {
					htmlContent.WriteString(fmt.Sprintf(`
<div style="text-align:center; margin-bottom:30px;">
  <img src="%s" alt="%s" style="width:100%%; max-height:100%%; object-fit:cover; border-radius:8px;">
  %s
</div>`,
						html.EscapeString(imageUrl),
						html.EscapeString(title),
						func() string {
							if title != "" {
								return fmt.Sprintf(`<h1 style="margin-top:16px; font-size:2.2em;">%s</h1>`, html.EscapeString(title))
							}
							return ""
						}(),
					))
				} else if title != "" {
					htmlContent.WriteString(fmt.Sprintf(`<h1 style="text-align:center; margin:30px 0;">%s</h1>`, html.EscapeString(title)))
				}

			default:
				// Для отладки: покажем неизвестный блок
				log.Printf("Неизвестный тип блока: %s, props: %+v", block.Type, block.Props)
				// Или покажем заглушку:
				htmlContent.WriteString(fmt.Sprintf(`<div style="color:red; padding:10px; border:1px dashed red;">Неизвестный блок: %s</div>`, html.EscapeString(block.Type)))
			}
		}

		fullHTML := fmt.Sprintf(`<!DOCTYPE html>
<html lang="ru">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>%s</title>
  <style>
    body {
      font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, Oxygen, sans-serif;
      line-height: 1.6;
      color: #333;
      background: #fff;
      margin: 0;
      padding: 20px;
      max-width: 800px;
      margin: 0 auto;
    }
    h1, h2, h3, h4, h5, h6 {
      margin-top: 1.2em;
      margin-bottom: 0.6em;
    }
    p {
      margin: 0 0 16px 0;
    }
    img {
      max-width: 100%%;
      height: auto;
    }
    a {
      color: #007bff;
    }
  </style>
</head>
<body>
  %s
</body>
</html>`, html.EscapeString(landing.Title), htmlContent.String())

		c.Header("Content-Type", "text/html; charset=utf-8")
		c.String(200, fullHTML)
	})

	// Обновить лендинг (только черновик!)
	r.PUT("/landings/:id", func(c *gin.Context) {
		id := c.Param("id")

		// Проверяем, существует ли лендинг
		existing, err := repository.GetLandingByID(id)
		if err != nil {
			c.JSON(404, gin.H{"error": "Landing not found"})
			return
		}

		// Парсим входящий JSON
		var updated models.Landing
		if err := c.ShouldBindJSON(&updated); err != nil {
			c.JSON(400, gin.H{"error": err.Error()})
			return
		}

		// Сохраняем оригинальные значения Published и PublicUrl
		// (чтобы нельзя было опубликовать лендинг через PUT)
		updated.ID = existing.ID
		updated.Published = existing.Published
		updated.PublicUrl = existing.PublicUrl

		// Обновляем в БД
		if err := repository.UpdateLanding(id, &updated); err != nil {
			c.JSON(500, gin.H{"error": "Update failed"})
			return
		}

		c.JSON(200, updated)
	})

	// Удалить лендинг (только если он не опубликован)
	r.DELETE("/landings/:id", func(c *gin.Context) {
		id := c.Param("id")

		// Проверяем существование и статус публикации
		landing, err := repository.GetLandingByID(id)
		if err != nil {
			c.JSON(404, gin.H{"error": "Landing not found"})
			return
		}

		if landing.Published {
			c.JSON(403, gin.H{
				"error": "Cannot delete published landing. Unpublish it first.",
			})
			return
		}

		// Удаляем из БД
		if err := repository.DeleteLandingByID(id); err != nil {
			log.Printf("Failed to delete landing %s: %v", id, err)
			c.JSON(500, gin.H{"error": "Internal server error"})
			return
		}

		c.Status(204) // Успешное удаление, без тела ответа
	})

	// Публичный просмотр (HTML — таблица управления лендингами)
	r.GET("/landings/table", func(c *gin.Context) {
		cursor, err := repository.GetAllLandings()
		if err != nil {
			c.JSON(500, gin.H{"error": "DB error"})
			return
		}
		defer cursor.Close(context.TODO())

		var landings []models.Landing
		if err = cursor.All(context.TODO(), &landings); err != nil {
			c.JSON(500, gin.H{"error": "Decode error"})
			return
		}

		var rows strings.Builder
		for _, l := range landings {
			id := l.ID.Hex()
			var publicCell, actionCell string

			if l.Published {
				publicURL := "/public/" + id
				publicCell = `<a href="` + html.EscapeString(publicURL) + `" target="_blank">` + html.EscapeString(publicURL) + `</a>`
				actionCell = `<button type="button" class="btn-unpublish" data-id="` + html.EscapeString(id) + `">Убрать из публикации</button>`
			} else {
				publicURL := "/landings/" + id
				publicCell = `<a href="` + html.EscapeString(publicURL) + `" target="_blank">` + html.EscapeString(publicURL) + `</a>`
				actionCell = `<button type="button" class="btn-publish" data-id="` + html.EscapeString(id) + `">Опубликовать</button>`
			}

			publishedStr := "Да"
			if !l.Published {
				publishedStr = "Нет"
			}

			rows.WriteString("<tr>")
			rows.WriteString("<td>" + html.EscapeString(id) + "</td>")
			rows.WriteString("<td>" + html.EscapeString(l.Title) + "</td>")
			rows.WriteString("<td>" + html.EscapeString(publishedStr) + "</td>")
			rows.WriteString("<td>" + publicCell + "</td>")
			rows.WriteString("<td>" + actionCell + "</td>")
			rows.WriteString("</tr>")
		}

		htmlPage := `<!DOCTYPE html>
<html>
<head>
  <meta charset="utf-8">
  <title>Все лендинги</title>
  <link rel="stylesheet" href="https://cdn.datatables.net/v/dt/dt-2.3.4/sb-1.8.4/datatables.min.css">
  <style>
    body { font-family: Arial, sans-serif; margin: 20px; }
    table { width: 100%; }
    .btn-publish, .btn-unpublish {
      display: inline-block;
      padding: 6px 12px;
      color: white;
      text-decoration: none;
      border: none;
      border-radius: 4px;
      font-size: 14px;
      cursor: pointer;
    }
    .btn-publish {
      background-color: #28a745;
    }
    .btn-publish:hover {
      background-color: #218838;
    }
    .btn-unpublish {
      background-color: #dc3545;
    }
    .btn-unpublish:hover {
      background-color: #c82333;
    }
    button:disabled {
      background-color: #ccc;
      cursor: not-allowed;
    }
  </style>
</head>
<body>
  <h1>Все лендинги</h1>
  <table id="landingsTable" class="display">
    <thead>
      <tr>
        <th>ID</th>
        <th>Title</th>
        <th>Published</th>
        <th>URL</th>
        <th>Действие</th>
      </tr>
    </thead>
    <tbody>
` + rows.String() + `
    </tbody>
  </table>

  <script src="https://code.jquery.com/jquery-3.7.1.min.js"></script>
  <script src="https://cdn.datatables.net/v/dt/dt-2.3.4/sb-1.8.4/datatables.min.js"></script>

  <script>
    $(document).ready(function() {
      new DataTable('#landingsTable', {
        pageLength: 25,
        language: {
          url: '//cdn.datatables.net/plug-ins/2.3.4/i18n/ru.json'
        },
        layout: {
          topStart: 'searchBuilder'
        }
      });

      // Опубликовать
      $(document).on('click', '.btn-publish', function() {
        const button = $(this);
        const id = button.data('id');
        if (!id) return;

        if (!confirm('Опубликовать лендинг с ID ' + id + '?')) {
          return;
        }

        button.prop('disabled', true).text('Публикуется...');

        $.post('/landings/' + id + '/publish')
          .done(function() {
            // Обновляем строку без перезагрузки
            const row = button.closest('tr');
            row.find('td:eq(2)').text('Да');
            row.find('td:eq(3)').html('<a href="/public/' + id + '" target="_blank">/public/' + id + '</a>');
            button.removeClass('btn-publish')
                  .addClass('btn-unpublish')
                  .text('Убрать из публикации')
                  .prop('disabled', false);
          })
          .fail(function() {
            alert('Ошибка при публикации!');
            button.prop('disabled', false).text('Опубликовать');
          });
      });

      // Убрать из публикации
      $(document).on('click', '.btn-unpublish', function() {
        const button = $(this);
        const id = button.data('id');
        if (!id) return;

        if (!confirm('Снять лендинг с публикации? ID: ' + id)) {
          return;
        }

        button.prop('disabled', true).text('Снимается...');

        $.post('/landings/' + id + '/unpublish')
          .done(function() {
            const row = button.closest('tr');
            row.find('td:eq(2)').text('Нет');
            row.find('td:eq(3)').html('<a href="/landings/' + id + '" target="_blank">/landings/' + id + '</a>');
            button.removeClass('btn-unpublish')
                  .addClass('btn-publish')
                  .text('Опубликовать')
                  .prop('disabled', false);
          })
          .fail(function() {
            alert('Ошибка при снятии с публикации!');
            button.prop('disabled', false).text('Убрать из публикации');
          });
      });
    });
  </script>
</body>
</html>`

		c.Data(200, "text/html; charset=utf-8", []byte(htmlPage))
	})

	r.Run(":8080")
}
