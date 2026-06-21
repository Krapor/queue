package main

import (
	"context"
	"crypto/tls"
	"fmt"
	"log"
	"net/http"
	"os"

	amqp "github.com/rabbitmq/amqp091-go"
	"github.com/redis/go-redis/v9"
)

var (
	ctx = context.Background()
	rdb *redis.Client
	ch  *amqp.Channel
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
			TLSConfig: &tls.Config{}, // Включаем TLS-шифрование, чтобы не было ошибки EOF
		})

		// Проверяем соединение
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
	rabbitURL := os.Getenv("RABBITMQ_URL")
	if rabbitURL != "" {
		conn, err := amqp.Dial(rabbitURL)
		if err != nil {
			log.Fatalf("[-] Ошибка подключения к RabbitMQ: %v", err)
		}
		defer conn.Close()

		ch, err = conn.Channel()
		if err != nil {
			log.Fatalf("[-] Ошибка открытия канала RabbitMQ: %v", err)
		}
		defer ch.Close()

		// Регистрируем очередь tasks
		_, err = ch.QueueDeclare(
			"tasks", // имя очереди
			true,    // durable
			false,   // delete when unused
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

	// 3. Запуск веб-сервера для Render (чтобы не падал по таймауту портов)
	port := os.Getenv("PORT")
	if port == "" {
		port = "10000"
	}

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "Сетевой Оркестратор работает. Стек: Go, Redis (Upstash), RabbitMQ (CloudAMQP)")
	})

	fmt.Printf("[*] Веб-сервер запущен на порту %s\n", port)
	if err := http.ListenAndServe(":"+port, nil); err != nil {
		log.Fatalf("[-] Ошибка запуска веб-сервера: %v", err)
	}
}
