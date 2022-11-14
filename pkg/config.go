package giga

import (
	"github.com/jinzhu/configor"
)

type NodeConfig struct {
	Host    string `default:"localhost"`
	ZMQPort string `default:"28332"`
	RPCPort int    `default:"44555"`
	RPCPass string `default:"gigawallet"`
	RPCUser string `default:"gigawallet"`
}

type Config struct {
	Gigawallet struct {
		// key for which Dogecoind struct to use
		Dogecoind           string `default:"testnet" required:"true" env:"network"`
		ConfirmationsNeeded int    `default:"60" required:"false"`
	}

	WebAPI struct {
		Port string `default:"8080" env:"port"`
	}

	Store struct {
		DBFile string `default:"gigawallet.db"`
	}

	// info for connecting to dogecoin-core daemon
	Dogecoind map[string]NodeConfig
}

func LoadConfig(confPath string) Config {
	c := Config{Dogecoind: make(map[string]NodeConfig)}
	configor.Load(&c, confPath)
	return c
}
