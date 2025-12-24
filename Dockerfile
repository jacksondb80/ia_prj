# ---------- BUILD ----------
FROM golang:1.24-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod tidy
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -o crawler ./cmd/crawler

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -o embeddings ./cmd/embeddings

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -o chat ./cmd/chat

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -o chatv2 ./cmd/chatv2

# ---------- RUNTIME ----------
FROM gcr.io/distroless/base-debian12

WORKDIR /app

COPY --from=builder /app/crawler /app/crawler
COPY --from=builder /app/embeddings /app/embeddings
COPY --from=builder /app/chat /app/chat
COPY --from=builder /app/chatv2 /app/chatv2
COPY --from=builder /app/views /app/views

EXPOSE 8080 8090 9090

# o comando final será definido no docker-compose
