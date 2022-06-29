package giga

import (
	"github.com/jinzhu/configor"
)

type Config struct {
	Gigawallet struct {
		// key for which Dogecoind struct to use
		Dogecoind string `default:"testnet" required:"true" env="network"`
	}

	// info for connecting to dogecoin-core daemon
	Dogecoind map[string]struct {
		Rpcaddr string `default:"localhost"`
		Rpcport int    `default:"44555"`
		Rpcpass string `default:"gigawallet"`
		Rpcuser string `default:"gigawallet"`
	}
}

func LoadConfig(confPath string) Config {
	c := Config{}
	configor.Load(&c, confPath)
	return c
}
