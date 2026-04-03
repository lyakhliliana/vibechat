FROM golang:1.24-alpine AS builder

WORKDIR /build

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 go build -ldflags="-s -w" -o /out/app  ./cmd/app


FROM alpine:3.20

RUN apk add --no-cache ca-certificates tzdata

COPY --from=builder /out/app  /app/services/vibechat/app
COPY configs/app.yaml         /app/services/vibechat/config.yaml

RUN mkdir -p /app/logs

EXPOSE 8080
EXPOSE 8082

CMD ["/app/services/vibechat/app", "-config", "/app/services/vibechat/config.yaml"]
