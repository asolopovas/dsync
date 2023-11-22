start:
	go run ./main.go

build:
	go build -o ./dist/dsync ./main.go
	chmod +x ./dist/dsync

install-local:
	go build -o $(GOBIN)/dsync ./main.go
	chmod +x $(GOBIN)/dsync

install:
	go install github.com/asolopovas/dsync@latest

test:
	 go run ./main.go -c ./dsync-config.json

