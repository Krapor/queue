
# --- ЭТАП 1: Сборка фронтенда (TypeScript) ---
FROM node:20-alpine AS frontend-builder
WORKDIR /app

# Копируем конфигурацию TS и исходники фронтенда
COPY tsconfig.json ./
COPY src/ ./src
COPY static/ ./static

# Устанавливаем компилятор и собираем TS в JS
RUN npm install -g typescript && tsc

# --- ЭТАП 2: Сборка бэкенда (Go) ---
FROM golang:1.26-alpine AS backend-builder
WORKDIR /app

# Копируем файлы модулей Go (если они есть в корне)
COPY go.mod* go.sum* ./
RUN go mod download || true

# Копируем код сервера и собираем бинарник
COPY main.go ./
RUN CGO_ENABLED=0 GOOS=linux go build -o server main.go

# --- ЭТАП 3: Финальный минимальный образ ---
FROM alpine:latest
WORKDIR /root/

# Забираем статику с готовым JS из первого этапа
COPY --from=frontend-builder /app/static ./static

# Забираем откомпилированный Go-сервер из второго этапа
COPY --from=backend-builder /app/server .

# Открываем порт для Render
EXPOSE 8080

# Запуск приложения
CMD ["./server"]
