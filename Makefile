build:
	CGO_ENABLED=0 go build -ldflags "-s -w"

clean:
	go clean
	rm -rf ./data ./imagehost-data ./imagehost.db

docker:
	docker build -t ayay2021/imagehost .

docker-push:
	docker push ayay2021/imagehost:latest