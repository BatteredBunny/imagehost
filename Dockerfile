FROM golang:1.24-alpine AS builder

WORKDIR /app

COPY go.mod .
COPY go.sum .
RUN go mod download

COPY . .

RUN go build -o /app/imagehost

FROM alpine:3.21

VOLUME [ "/app/data" ]
EXPOSE 80
WORKDIR /app

COPY --from=builder /app/imagehost /app/imagehost

ENTRYPOINT [ "/app/imagehost", "-c", "/app/config.toml" ]