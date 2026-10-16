FROM golang:1.25.5-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 go build -o server ./cmd/server

FROM alpine:latest

RUN apk add --no-cache ca-certificates && adduser -D app

WORKDIR /app

COPY --from=builder /app/server .

USER app

EXPOSE 8080

CMD ["./server"]
