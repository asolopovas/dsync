start:
	go run ./main.go

build:
	go build -o ./dist/dsync ./main.go
	chmod +x ./dist/dsync
