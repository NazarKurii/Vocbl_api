
BINARY=bin/vocbl_api


build:
	@go build -o $(BINARY)


run: build
	@./$(BINARY)


test:
	@go test -v ./...
