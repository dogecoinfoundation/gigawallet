default: build/gigawallet

.PHONY: clean, test
clean:
	rm -rf ./build

build/gigawallet: clean
	mkdir -p build/
	go build -o build/gigawallet ./cmd/gigawallet/.

# useful to fix cached CGO linker paths
# "ld: warning: directory not found"; "ld: library not found"
build-all-deps: clean
	mkdir -p build/
	go build -a -o build/gigawallet ./cmd/gigawallet/.

# run the built gigawallet (uses config.toml)
# you will need to create config.toml
run: build/gigawallet
	./build/gigawallet server

# runs without building first (uses devconf.toml)
dev:
	GIGA_ENV=devconf go run ./cmd/gigawallet/. server 

test:
	go test -v ./pkg/doge
	go test -v ./pkg/webapi
	go test -v ./test
