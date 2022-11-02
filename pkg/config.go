package giga

import (
	"github.com/jinzhu/configor"
)

type Config struct {
	Gigawallet struct {
		// key for which Dogecoind struct to use
		Dogecoind           string `default:"testnet" required:"true" env:"network"`
		ConfirmationsNeeded int    `default:"60" required:"false"`
	}

	WebAPI struct {
		Port string `default:"8080" env:"port"`
	}

	// info for connecting to dogecoin-core daemon
	Dogecoind map[string]struct {
		Host    string `default:"localhost"`
		ZMQPort string `default:"28332"`
		RPCPort int    `default:"44555"`
		RPCPass string `default:"gigawallet"`
		RPCUser string `default:"gigawallet"`
	}
}

func LoadConfig(confPath string) Config {
	c := Config{}
	configor.Load(&c, confPath)
	return c
}
