default: build/gigawallet

.PHONY: clean, test
clean:
	rm -rf ./build

build/gigawallet: clean
	mkdir -p build/
	go build -o build/gigawallet ./cmd/gigawallet/. 


dev:
	GIGA_ENV=devconf go run ./cmd/gigawallet/. server 


test:
	go test -v ./pkg/doge
	go test -v ./test
