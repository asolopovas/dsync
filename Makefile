start:
	go run .

build:
	go build -o ./dist/dsync .
	chmod +x ./dist/dsync

install-local:
	go build -o $(GOBIN)/dsync .
	chmod +x $(GOBIN)/dsync

install:
	go install github.com/asolopovas/dsync@latest

test:
	 go run . -c ./dsync-config.json

tag-push:
	$(eval VERSION=$(shell cat version))
	git tag $(VERSION)
	git push origin $(VERSION)
	if git rev-parse latest >/dev/null 2>&1; then git tag -d latest; fi
	git tag latest
	git push origin latest --force
