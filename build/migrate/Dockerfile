FROM golang:1.24-alpine AS builder

WORKDIR /build

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 go build -ldflags="-s -w" -o /out/migrate ./cmd/migrate


FROM alpine:3.20

RUN apk add --no-cache ca-certificates tzdata

COPY --from=builder /out/migrate /app/services/migrate/migrate
COPY configs/migrate.yaml        /app/services/migrate/config.yaml
COPY migrations                  /app/services/migrate/migrations

RUN mkdir -p /app/logs

CMD ["/app/services/migrate/migrate", "-config", "/app/services/migrate/config.yaml"]
