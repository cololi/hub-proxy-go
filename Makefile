.PHONY: build test clean run

BINARY_NAME=hub-proxy-go
DIST_DIR=bin

build:
	go build -ldflags="-s -w" -trimpath -o $(BINARY_NAME) ./cmd/hub-proxy-go

test:
	go test -v ./...

clean:
	rm -f $(BINARY_NAME)
	rm -rf $(DIST_DIR)
	rm -f *.log
	rm -f *.test

run: build
	./$(BINARY_NAME)

dist:
	mkdir -p $(DIST_DIR)
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -trimpath -o $(DIST_DIR)/$(BINARY_NAME)-linux-amd64 ./cmd/hub-proxy-go
	CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -ldflags="-s -w" -trimpath -o $(DIST_DIR)/$(BINARY_NAME)-linux-arm64 ./cmd/hub-proxy-go
	CGO_ENABLED=0 GOOS=darwin GOARCH=arm64 go build -ldflags="-s -w" -trimpath -o $(DIST_DIR)/$(BINARY_NAME)-darwin-arm64 ./cmd/hub-proxy-go
	CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -ldflags="-s -w" -trimpath -o $(DIST_DIR)/$(BINARY_NAME)-windows-amd64.exe ./cmd/hub-proxy-go
