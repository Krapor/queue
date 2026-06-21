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
	"google.golang.org/api/drive/v3"
	"google.golang.org/api/option"
)

var (
	ctx        = context.Background()
	rdb        *redis.Client
	amqpConn   *amqp.Connection
	amqpChan   *amqp.Channel
	rabbitURL  string
	driveService *drive.Service // Глобальный клиент для работы с Google Диском
)

func main() {
	fmt.Println("=== Запуск Сетевого Оркестратора в Docker-контейнере ===")

	// 1. Инициализация Redis с поддержкой TLS для Upstash
	initRedis()

	// 2. Инициализация RabbitMQ
	initRabbitMQ()

	// 3. Инициализация Google Drive API (Проверка секретного JSON-ключа)
	initGoogleDrive()

	// 4. Запуск веб-сервера и маршрутов
	port := os.Getenv("PORT")
	if port == "" {
		port = "10000"
	}

	http.HandleFunc("/", handleIndex)
	http.HandleFunc("/send-task", handleSendTask)

	fmt.Printf("[*] Веб-сервер запущен на порту %s\n", port)
	if err := http.ListenAndServe(":"+port, nil); err != nil {
		log.Fatalf("[-] Ошибка запуска веб-сервера: %v", err)
	}
}

// --- БЛОК ИНИЦИАЛИЗАЦИИ GOOGLE DRIVE API ---
func initGoogleDrive() {
	// Путь внутри контейнера, куда мы будем прокидывать секретный ключ
	credPath := "/app/credentials/google-key.json"

	// Проверяем, существует ли файл физически
	if _, err := os.Stat(credPath); os.IsNotExist(err) {
		log.Println("[Предупреждение] Файл google-key.json не найден. Работа с Colab/Drive API отключена.")
		return
	}

	// Если файл на месте, пытаемся авторизоваться в Google
	log.Println("[*] Обнаружен ключ Google. Попытка авторизации через Service Account...")
	
	service, err := drive.NewService(ctx, option.WithCredentialsFile(credPath), option.WithScopes(drive.DriveScope))
	if err != nil {
		log.Printf("[-] Ошибка авторизации в Google Drive API: %v", err)
		return
	}

	driveService = service
	fmt.Println("[+] Авторизация в Google Drive API прошла успешно! Доступ к блокнотам открыт.")
}

// --- ОСТАЛЬНЫЕ СЛУЖЕБНЫЕ ФУНКЦИИ СТЕКА ---

func initRedis() {
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
}

func initRabbitMQ() {
	rabbitURL = os.Getenv("RABBITMQ_URL")
	if rabbitURL != "" {
		var err error
		amqpConn, err = amqp.Dial(rabbitURL)
		if err != nil {
			log.Fatalf("[-] Ошибка подключения к RabbitMQ: %v", err)
		}

		amqpChan, err = amqpConn.Channel()
		if err != nil {
			log.Fatalf("[-] Ошибка открытия канала RabbitMQ: %v", err)
		}

		_, err = amqpChan.QueueDeclare("tasks", true, false, false, false, nil)
		if err != nil {
			log.Fatalf("[-] Ошибка декларации очереди: %v", err)
		}
		fmt.Println("[+] Очередь RabbitMQ 'tasks' успешно запущена и слушает!")
	} else {
		log.Println("[Предупреждение] Переменная RABBITMQ_URL не задана")
	}
}

func handleIndex(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	
	// Проверяем статус Google Drive для отображения на странице
	googleStatus := "<span style='color: red;'>Отключено (нет JSON-ключа)</span>"
	if driveService != nil {
		googleStatus = "<span style='color: green;'>Активно (Связь с Google Диском установлена)</span>"
	}

	html := fmt.Sprintf(`
	<!DOCTYPE html>
	<html>
	<head>
		<title>Панель управления Оркестратором</title>
		<style>
			body { font-family: Arial, sans-serif; text-align: center; margin-top: 50px; background-color: #f4f7f6; }
			.btn { background-color: #24a0ed; color: white; padding: 15px 30px; font-size: 18px; border: none; border-radius: 5px; cursor: pointer; box-shadow: 0 4px 6px rgba(0,0,0,0.1); transition: 0.2s; }
			.btn:hover { background-color: #0d8bf2; }
			#status { margin-top: 20px; font-weight: bold; color: #333; }
			.info { margin-bottom: 30px; font-size: 14px; color: #666; }
		</style>
		<script>
			function sendTask() {
				document.getElementById("status").innerText = "Отправка задачи...";
				fetch("/send-task")
					.then(response => response.text())
					.then(data => { document.getElementById("status").innerText = data; })
					.catch(err => { document.getElementById("status").innerText = "Ошибка отправки"; });
			}
		</script>
	</head>
	<body>
		<h1>Сетевой Оркестратор Go в Docker</h1>
		<div class="info">Статус Google Drive API: %s</div>
		<p>Нажмите кнопку ниже, чтобы отправить вычислительную задачу в RabbitMQ</p>
		<button class="btn" onclick="sendTask()">⚡ Запустить расчет на TPU/GPU</button>
		<div id="status">Ожидание действий...</div>
	</body>
	</html>`, googleStatus)
	fmt.Fprint(w, html)
}

func handleSendTask(w http.ResponseWriter, r *http.Request) {
	if amqpChan == nil {
		http.Error(w, "RabbitMQ не подключен", http.StatusInternalServerError)
		return
	}

	rand.Seed(time.Now().UnixNano())
	taskID := fmt.Sprintf("TASK-%d", rand.Intn(900000)+100000)

	taskJSON := fmt.Sprintf(`{"task_id": "%s", "timestamp": "%s", "command": "run_matrix_multiplication"}`, 
		taskID, time.Now().Format(time.RFC3339))

	err := amqpChan.PublishWithContext(ctx, "", "tasks", false, false,
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
