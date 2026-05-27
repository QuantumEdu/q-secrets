.PHONY: build build-all test clean run

BINARY=q-secret
BIN_DIR=bin

build:
	go build -o $(BINARY) .

build-all:
	GOOS=linux GOARCH=amd64 go build -o $(BIN_DIR)/$(BINARY)-linux .
	GOOS=darwin GOARCH=amd64 go build -o $(BIN_DIR)/$(BINARY)-darwin .
	GOOS=windows GOARCH=amd64 go build -o $(BIN_DIR)/$(BINARY).exe .

test:
	go test ./...

clean:
	rm -rf $(BIN_DIR) $(BINARY) $(BINARY).exe *.db public.key

run: build
	$(BINARY) run

help:
	@echo "Targets: build, build-all, test, clean, run"
