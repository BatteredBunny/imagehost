FROM golang:1.25-alpine AS builder

WORKDIR /app

COPY go.mod .
COPY go.sum .
RUN go mod download

COPY . .

RUN go build -o /app/hostling

FROM alpine:3.23

VOLUME [ "/app/data" ]
EXPOSE 80
WORKDIR /app

COPY --from=builder /app/hostling /app/hostling

ENTRYPOINT [ "/app/hostling", "-c", "/app/config.toml" ]