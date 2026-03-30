BINARY := lazydk
GOFLAGS := -ldflags="-s -w"

.PHONY: build run clean tidy

build:
	go build $(GOFLAGS) -o $(BINARY) ./cmd/lazydk/

run: build
	./$(BINARY)

clean:
	rm -f $(BINARY)

tidy:
	go mod tidy
