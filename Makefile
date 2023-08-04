start:
	go run ./main.go

build:
	go build -o ./dist/dsync ./main.go
	chmod +x ./dist/dsync

install:
	go install github.com/asolopovas/dsync@latest
