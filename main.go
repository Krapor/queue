package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
	"github.com/redis/go-redis/v9"
)

var ctx = context.Background()

// Структуры для Gemini API
type GeminiRequest struct {
	Contents []Content `json:"contents"`
}
type Content struct {
	Parts []Part `json:"parts"`
}
type Part struct {
	Text string `json:"text"`
}
type GeminiResponse struct {
	Candidates []Candidate `json:"candidates"`
}
type Candidate struct {
	Content struct {
		Parts []Part `json:"parts"`
	} `json:"content"`
}

func askGemini(apiKey string, prompt string) string {
	url := fmt.Sprintf("https://generativelanguage.googleapis.com/v1beta/models/gemini-1.5-flash:generateContent?key=%s", apiKey)
	reqBody := GeminiRequest{Contents: []Content{{Parts: []Part{{Text: prompt}}}}}
	jsonData, _ := json.Marshal(reqBody)

	resp, err := http.Post(url, "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Sprintf("Ошибка сети Gemini: %v", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	var geminiResp GeminiResponse
	json.Unmarshal(body, &geminiResp)

	if len(geminiResp.Candidates) > 0 && len(geminiResp.Candidates[0].Content.Parts) > 0 {
		return geminiResp.Candidates[0].Content.Parts[0].Text
	}
	return "ИИ не вернул текст."
}

func main() {
	log.Println("=== Запуск Сетевого Оркестратора со всем стеком ===")

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	geminiKey := os.Getenv("GEMINI_API_KEY")
	redisAddr := os.Getenv("REDIS_URL")
	redisPassword := os.Getenv("REDIS_PASSWORD")
	rabbitmqURL := os.Getenv("RABBITMQ_URL")

	// 1. Настройка Redis
	var rdb *redis.Client
	if redisAddr != "" {
		rdb = redis.NewClient(&redis.Options{
			Addr:     redisAddr,
			Password: redisPassword,
		})
		if _, err := rdb.Ping(ctx).Result(); err != nil {
			log.Printf("[Предупреждение] Redis недоступен: %v", err)
		} else {
			log.Println("[+] Успешное подключение к Redis!")
		}
	}

	// 2. Настройка RabbitMQ в фоне
	if rabbitmqURL != "" {
		go func() {
			conn, err := amqp.Dial(rabbitmqURL)
			if err != nil {
				log.Printf("[Ошибка] RabbitMQ: %v", err)
				return
			}
			defer conn.Close()

			ch, _ := conn.Channel()
			defer ch.Close()

			q, _ := ch.QueueDeclare("tasks", true, false, false, false, nil)
			msgs, _ := ch.Consume(q.Name, "", true, false, false, false, nil)

			log.Println("[+] Очередь RabbitMQ 'tasks' успешно запущена и слушает!")

			for d := range msgs {
				taskText := string(d.Body)
				log.Printf("[Очередь] Получена задача: %s", taskText)

				// Логика автоматизации: отправляем текст задачи в Gemini для обработки
				aiAnalysis := askGemini(geminiKey, "Проанализируй эту задачу автоматизации и выдели суть за 1 фразу: "+taskText)
				log.Printf("[ИИ Анализ]: %s", aiAnalysis)

				// Сохраняем результат анализа в кэш Redis
				if rdb != nil {
					rdb.Set(ctx, "last_processed_task", aiAnalysis, 30*time.Minute)
					log.Println("[Кэш] Результат сохранен в Redis.")
				}
			}
		}()
	}

	// 3. Веб-интерфейс
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")

		fmt.Fprintf(w, "=== СЕТЕВОЙ ОРКЕСТРАТОР АВТОМАТИЗАЦИИ ===\n")
		fmt.Fprintf(w, "Статус системы: Полная интеграция (LIVE)\n")
		fmt.Fprintf(w, "Время сервера: %s\n\n", time.Now().Format("15:04:05"))

		if rdb != nil {
			lastTask, err := rdb.Get(ctx, "last_processed_task").Result()
			if err == nil {
				fmt.Fprintf(w, "Последний результат из кэша Redis:\n%s\n", lastTask)
			} else {
				fmt.Fprintf(w, "Кэш Redis пуст (ожидание задач из RabbitMQ).\n")
			}
		}

		if geminiKey != "" {
			fmt.Fprintf(w, "\nТестовый пинг ИИ...\n")
			fmt.Fprintf(w, "Девиз от Gemini: %s", askGemini(geminiKey, "Напиши девиз из 3 слов для этого оркестратора."))
		}
	})

	log.Printf("[*] Веб-сервер запущен на порту %s", port)
	if err := http.ListenAndServe(":"+port, nil); err != nil {
		log.Fatalf("[-] Ошибка запуска: %v", err)
	}
}
