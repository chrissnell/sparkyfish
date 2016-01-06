all: sparkyfish-server

sparkyfish-server:
	go build

docker: Dockerfile
	GOOS=linux GOARCH=amd64 go build -o sparkyfish-server-linux-amd64
	docker build .
