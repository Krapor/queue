# Этап фронтенда
FROM node:20-alpine AS frontend-builder
WORKDIR /app
COPY tsconfig.json ./
COPY src/ ./src
COPY static/ ./static
RUN npm install -g typescript && tsc

# Этап бэкенда
FROM golang:1.23-alpine AS backend-builder
WORKDIR /app
COPY go.mod* go.sum* ./
RUN go mod download || true
COPY main.go ./
RUN CGO_ENABLED=0 GOOS=linux go build -o server main.go

# Финал
FROM alpine:latest
WORKDIR /root/
COPY --from=frontend-builder /app/static ./static
COPY --from=backend-builder /app/server .
EXPOSE 8080
CMD ["./server"]
