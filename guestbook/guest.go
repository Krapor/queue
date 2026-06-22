FROM golang:1.26-alpine AS builder
WORKDIR /app
# Копируем go.mod из корня проекта для подтягивания зависимостей
COPY go.mod ./
RUN go mod download
# Копируем код именно из папки guest
COPY guest/main.go ./
RUN CGO_ENABLED=0 GOOS=linux go build -o guest-server main.go

FROM alpine:latest
RUN apk --no-cache add ca-certificates
WORKDIR /root/
COPY --from=builder /app/guest-server .
EXPOSE 8080
CMD ["./guest-server"]

