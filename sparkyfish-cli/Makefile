all: sparkyfish-cli

sparkyfish-cli:
	go build

docker-build: Dockerfile
	GOOS=linux GOARCH=amd64 go build -o sparkyfish-cli-linux-amd64
	docker build -t chrissnell/sparkyfish-cli:$(GIT_TAG) .

docker-push:
	docker push chrissnell/sparkyfish-cli:$(GIT_TAG)
	docker tag -f chrissnell/sparkyfish-cli:$(GIT_TAG) chrissnell/sparkyfish-cli:latest
	docker push chrissnell/sparkyfish-cli:latest

docker: docker-build docker-push

GIT_TAG := $(shell git describe --tag --abbrev=0)
