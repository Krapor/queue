port := os.Getenv("PORT")
if port == "" {
    port = "8080" // Локальный порт по умолчанию
}
log.Fatal(http.ListenAndServe(":"+port, nil))
