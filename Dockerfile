FROM golang:1.17

VOLUME [ "/app/data" ]
EXPOSE 80
WORKDIR /app

COPY go.mod .
COPY go.sum .
RUN go mod download

COPY public/ public/
COPY template/ template/
COPY auto_deletion.go .
COPY main.go .
COPY api.go .
COPY admin.go .

RUN go build -ldflags "-s -w" -o ./imagehost
RUN rm go.mod go.sum main.go auto_deletion.go api.go admin.go

ENTRYPOINT [ "/app/imagehost -c /app/config.json" ]