module github.com/dogecoinfoundation/gigawallet

go 1.18

require (
	github.com/dogeorg/go-libdogecoin v0.0.42
	github.com/jinzhu/configor v1.2.1
	github.com/julienschmidt/httprouter v1.3.0
	github.com/mattn/go-sqlite3 v1.14.14
	github.com/pebbe/zmq4 v1.2.9
	github.com/shopspring/decimal v1.3.1
	github.com/tjstebbing/conductor v1.0.0
)

require (
	github.com/BurntSushi/toml v0.3.1 // indirect
	gopkg.in/natefinch/lumberjack.v2 v2.0.0 // indirect
	gopkg.in/yaml.v2 v2.2.2 // indirect
)

replace github.com/dogeorg/go-libdogecoin => /Users/tjstebbing/code/go-libdogecoin
