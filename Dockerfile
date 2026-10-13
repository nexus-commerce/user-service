FROM golang:1.27.2-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./

RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -o main ./src/cmd/main/main.go

FROM alpine:latest

WORKDIR /root/

COPY --from=builder /app/main .

RUN apk add --no-cache ca-certificates

EXPOSE 80
CMD ["./main"]
