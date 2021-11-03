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

RUN go build .
RUN rm go.mod go.sum main.go auto_deletion.go api.go

ENTRYPOINT [ "/app/imagehost" ]