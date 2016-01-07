all: sparkyfish-server

sparkyfish-server:
	go build

docker-build: Dockerfile
	GOOS=linux GOARCH=amd64 go build -o sparkyfish-server-linux-amd64
	docker build -t chrissnell/sparkyfish-server:$(GIT_TAG) .

docker-push:
	docker push chrissnell/sparkyfish-server:$(GIT_TAG)
	docker tag -f chrissnell/sparkyfish-server:$(GIT_TAG) chrissnell/sparkyfish-server:latest
	docker push chrissnell/sparkyfish-server:latest

docker: docker-build docker-push

GIT_TAG := $(shell git describe --tag --abbrev=0)
