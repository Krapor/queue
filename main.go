package main

import (
	"context"
	"crypto/tls"
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"os"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
	"github.com/redis/go-redis/v9"
)

var (
	ctx       = context.Background()
	rdb       *redis.Client
	amqpConn  *amqp.Connection
	amqpChan  *amqp.Channel
	rabbitURL string
)

func main() {
	fmt.Println("=== Запуск Сетевого Оркестратора со всем стеком ===")

	// 1. Инициализация Redis с поддержкой TLS для Upstash
	redisAddr := os.Getenv("REDIS_URL")
	redisPassword := os.Getenv("REDIS_PASSWORD")

	if redisAddr != "" {
		rdb = redis.NewClient(&redis.Options{
			Addr:      redisAddr,
			Password:  redisPassword,
			TLSConfig: &tls.Config{},
		})

		err := rdb.Ping(ctx).Err()
		if err != nil {
			log.Printf("[Предупреждение] Redis недоступен: %v", err)
		} else {
			fmt.Println("[+] Успешное подключение к Redis!")
		}
	} else {
		log.Println("[Предупреждение] Переменные REDIS_URL не заданы")
	}

	// 2. Инициализация RabbitMQ
	rabbitURL = os.Getenv("RABBITMQ_URL")
	if rabbitURL != "" {
		var err error
		amqpConn, err = amqp.Dial(rabbitURL)
		if err != nil {
			log.Fatalf("[-] Ошибка подключения к RabbitMQ: %v", err)
		}
		defer amqpConn.Close()

		amqpChan, err = amqpConn.Channel()
		if err != nil {
			log.Fatalf("[-] Ошибка открытия канала RabbitMQ: %v", err)
		}
		defer amqpChan.Close()

		_, err = amqpChan.QueueDeclare(
			"tasks", // имя очереди
			true,    // durable
			false,   // auto-delete
			false,   // exclusive
			false,   // no-wait
			nil,     // arguments
		)
		if err != nil {
			log.Fatalf("[-] Ошибка декларации очереди: %v", err)
		}
		fmt.Println("[+] Очередь RabbitMQ 'tasks' успешно запущена и слушает!")
	} else {
		log.Println("[Предупреждение] Переменная RABBITMQ_URL не задана")
	}

	// 3. Запуск веб-сервера и маршрутов
	port := os.Getenv("PORT")
	if port == "" {
		port = "10000"
	}

	// Главная страница с кнопкой
	http.HandleFunc("/", handleIndex)
	// Эндпоинт для отправки задачи
	http.HandleFunc("/send-task", handleSendTask)

	fmt.Printf("[*] Веб-сервер запущен на порту %s\n", port)
	if err := http.ListenAndServe(":"+port, nil); err != nil {
		log.Fatalf("[-] Ошибка запуска веб-сервера: %v", err)
	}
}

// Отображает красивую кнопку в браузере
func handleIndex(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	html := `
	<!DOCTYPE html>
	<html>
	<head>
		<title>Панель управления Оркестратором</title>
		<style>
			body { font-family: Arial, sans-serif; text-align: center; margin-top: 100px; background-color: #f4f7f6; }
			.btn { background-color: #24a0ed; color: white; padding: 15px 30px; font-size: 18px; border: none; border-radius: 5px; cursor: pointer; box-shadow: 0 4px 6px rgba(0,0,0,0.1); transition: 0.2s; }
			.btn:hover { background-color: #0d8bf2; }
			#status { margin-top: 20px; font-weight: bold; color: #333; }
		</style>
		<script>
			function sendTask() {
				document.getElementById("status").innerText = "Отправка задачи...";
				fetch("/send-task")
					.then(response => response.text())
					.then(data => {
						document.getElementById("status").innerText = data;
					})
					.catch(err => {
						document.getElementById("status").innerText = "Ошибка отправки";
					});
			}
		</script>
	</head>
	<body>
		<h1>Сетевой Оркестратор Go</h1>
		<p>Нажмите кнопку ниже, чтобы отправить вычислительную задачу в RabbitMQ для Google Colab</p>
		<button class="btn" onclick="sendTask()">⚡ Запустить расчет на TPU/GPU</button>
		<div id="status">Ожидание действий...</div>
	</body>
	</html>`
	fmt.Fprint(w, html)
}

// Кнопка вызывает этот метод, и Go отправляет JSON в RabbitMQ
func handleSendTask(w http.ResponseWriter, r *http.Request) {
	if amqpChan == nil {
		http.Error(w, "RabbitMQ не подключен", http.StatusInternalServerError)
		return
	}

	// Формируем уникальный ID задачи
	rand.Seed(time.Now().UnixNano())
	taskID := fmt.Sprintf("TASK-%d", rand.Intn(900000)+100000)

	// Наш JSON-пакет для Google Colab
	taskJSON := fmt.Sprintf(`{"task_id": "%s", "timestamp": "%s", "command": "run_matrix_multiplication"}`, 
		taskID, time.Now().Format(time.RFC3339))

	// Публикуем в очередь 'tasks'
	err := amqpChan.PublishWithContext(ctx,
		"",      // exchange
		"tasks", // routing key (имя очереди)
		false,   // mandatory
		false,   // immediate
		amqp.Publishing{
			ContentType: "application/json",
			Body:        []byte(taskJSON),
		},
	)

	if err != nil {
		log.Printf("[-] Ошибка отправки сообщения в RabbitMQ: %v", err)
		fmt.Fprintf(w, "Ошибка отправки в очередь: %v", err)
		return
	}

	log.Printf("[+] В очередь отправлена задача: %s", taskID)
	fmt.Fprintf(w, "Успешно отправлено! Задача: %s", taskID)
}
