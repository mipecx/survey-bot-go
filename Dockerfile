FROM golang:1.26-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
# Собираем бинарник (путь cmd/bot/main.go)
RUN go build -o bot ./cmd/bot/main.go

FROM alpine:latest
WORKDIR /root/
# Копируем бинарник из билдера
COPY --from=builder /app/bot .

CMD ["./bot"]
