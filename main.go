package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"time"
)

func main() {
	log.Println("=== Старт микросервиса на Go ===")

	// Берем порт, который дает Render, или 8080 по умолчанию
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	// Простейший обработчик запросов
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "Привет! Микросервер Go успешно запущен в облаке. Время: %s", time.Now().Format("15:04:05"))
	})

	log.Printf("[*] Веб-сервер слушает порт %s", port)
	
	// Запуск сервера (эта строка заблокирует выполнение и будет держать сервис активным)
	if err := http.ListenAndServe(":"+port, nil); err != nil {
		log.Fatalf("[-] Ошибка запуска сервера: %v", err)
	}
}
