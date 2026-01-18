# Build stage
FROM golang:1.25.5-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./

RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -o navifetch .

FROM alpine:latest

RUN apk add --no-cache \
    ffmpeg \
    python3 \
    ca-certificates \
    curl

RUN curl -L https://github.com/yt-dlp/yt-dlp/releases/latest/download/yt-dlp -o /usr/local/bin/yt-dlp && \
    chmod a+rx /usr/local/bin/yt-dlp

WORKDIR /app

COPY --from=builder /app/navifetch .

EXPOSE 8080

CMD ["./navifetch"]
