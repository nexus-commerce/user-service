FROM golang:1.27.2-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./

RUN go mod download

COPY . .

# One image, several entrypoints: api (default), relay, consumer, purge.
RUN for cmd in api relay consumer purge; do \
        CGO_ENABLED=0 GOOS=linux go build -o /out/$cmd ./cmd/$cmd || exit 1; \
    done

FROM alpine:latest

WORKDIR /root/

COPY --from=builder /out/ /usr/local/bin/

RUN apk add --no-cache ca-certificates

EXPOSE 80
CMD ["api"]
