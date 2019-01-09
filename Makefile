install: vendor
	go install ./...

build: vendor
	go build -o bin/spoot main.go

vendor:
	go mod vendor

.PHONY: vendor build
