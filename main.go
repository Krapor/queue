package main // Это ОБЯЗАТЕЛЬНО должно быть на 1-й строке

import (
	"fmt"
	"log"
	"net/http"
	"os"
)

func main() {
	// Вот ПЕРЕД этой переменной port не должно быть никакого другого кода
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
    
	// ... дальше ваш остальной код подключения к Redis/RabbitMQ
}
