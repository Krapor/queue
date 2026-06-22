
package main

import (
	"context"
	"crypto/tls"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"os"

	"github.com/redis/go-redis/v9"
)

var (
	ctx = context.Background()
	rdb *redis.Client
)

// Структура для отзыва
type Comment struct {
	Name    string
	Message string
}

func main() {
	fmt.Println("=== Запуск Контейнера Гостевой Страницы ===")

	// Подключаемся к тому же Redis (Upstash)
	initRedis()

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080" // Этот контейнер будет работать на другом порту
	}

	http.HandleFunc("/", handleGuestPage)
	http.HandleFunc("/submit", handleSubmit)

	fmt.Printf("[*] Гостевая страница доступна на порту %s\n", port)
	if err := http.ListenAndServe(":"+port, nil); err != nil {
		log.Fatalf("[-] Ошибка сервера: %v", err)
	}
}

func initRedis() {
	redisAddr := os.Getenv("REDIS_URL")
	redisPassword := os.Getenv("REDIS_PASSWORD")

	if redisAddr != "" {
		rdb = redis.NewClient(&redis.Options{
			Addr:      redisAddr,
			Password:  redisPassword,
			TLSConfig: &tls.Config{},
		})
		if err := rdb.Ping(ctx).Err(); err != nil {
			log.Printf("[!] Redis недоступен для гостевой: %v", err)
		} else {
			fmt.Println("[+] Гостевая страница успешно подключилась к Redis!")
		}
	}
}

func handleGuestPage(w http.ResponseWriter, r *http.Request) {
	// Достаем последние 10 комментариев из списка Redis
	var comments []Comment
	if rdb != nil {
		savedComments, _ := rdb.LRange(ctx, "guestbook", 0, 9).Result()
		for _, text := range savedComments {
			// Простейший парсинг строки формата "Имя: Сообщение"
			var c Comment
			_, err := fmt.Sscanf(text, "%s: %s", &c.Name, &c.Message)
			if err == nil || text != "" {
				// Если sscanf не справился с пробелами, запишем текст целиком в сообщение
				comments = append(comments, Comment{Name: "Гость", Message: text})
			}
		}
	}

	// Простой HTML-шаблон прямо в коде
	tmplSrc := `
	<!DOCTYPE html>
	<html>
	<head>
		<title>Гостевая Книга</title>
		<style>
			body { font-family: 'Segoe UI', Arial, sans-serif; max-width: 600px; margin: 50px auto; padding: 20px; background-color: #fafafa; }
			.card { background: white; padding: 20px; border-radius: 8px; box-shadow: 0 2px 4px rgba(0,0,0,0.1); margin-bottom: 20px; }
			input, textarea { width: 95%; padding: 10px; margin: 10px 0; border: 1px solid #ccc; border-radius: 4px; }
			button { background-color: #2ecc71; color: white; padding: 10px 20px; border: none; border-radius: 4px; cursor: pointer; font-size: 16px; }
			button:hover { background-color: #27ae60; }
			.comment { border-left: 4px solid #2ecc71; padding-left: 10px; margin: 10px 0; background: #f9f9f9; padding: 5px 10px; }
		</style>
	</head>
	<body>
		<div class="card">
			<h2>📝 Оставьте свой отзыв</h2>
			<form action="/submit" method="POST">
				<input type="text" name="name" placeholder="Ваше имя" required><br>
				<textarea name="message" placeholder="Ваше сообщение" rows="4" required></textarea><br>
				<button type="submit">Отправить в систему</button>
			</form>
		</div>

		<div class="card">
			<h3>Последние отзывы:</h3>
			{{range .}}
				<div class="comment">
					<strong>{{.Message}}</strong>
				</div>
			{{else}}
				<p style="color: #888;">Отзывов пока нет. Будьте первым!</p>
			{{end}}
		</div>
		<p style="text-align: center;"><a href="/" style="color: #888; text-decoration: none;">← Назад на главную</a></p>
	</body>
	</html>`

	tmpl, _ := template.New("guest").Parse(tmplSrc)
	tmpl.Execute(w, comments)
}

func handleSubmit(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}

	name := r.FormValue("name")
	message := r.FormValue("message")

	if name != "" && message != "" && rdb != nil {
		entry := fmt.Sprintf("%s: %s", name, message)
		// Сохраняем в начало списка "guestbook" в Redis
		rdb.LPush(ctx, "guestbook", entry)
		// Обрезаем список до 50 записей, чтобы не переполнять базу
		rdb.LTrim(ctx, "guestbook", 0, 49)
	}

	// Возвращаем пользователя обратно на гостевую страницу
	http.Redirect(w, r, "/", http.StatusSeeOther)
}
