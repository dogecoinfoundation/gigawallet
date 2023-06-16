package giga

import (
	"fmt"

	"github.com/jinzhu/configor"
)

type NodeConfig struct {
	Host    string `default:"localhost"`
	ZMQPort int    `default:"28332"`
	RPCHost string `default:"127.0.0.1"`
	RPCPort int    `default:"44555"`
	RPCPass string `default:"gigawallet"`
	RPCUser string `default:"gigawallet"`
}

type Config struct {
	Gigawallet struct {
		// Keys used by Doge Connect to identify and verify this
		// Gigawallet instance.
		ServiceName string `default:"Example Dogecoin Store"`
		// ServiceDomain will be used to lookup the ServiceKey from
		// DNS TXT record and verified against ServiceKeyHash.
		ServiceDomain  string `default:"example.com"`
		ServiceIconURL string `default:"https://example.com/icon.png"`
		ServiceKeyHash string `default:""`

		// key for which Dogecoind struct to use
		Dogecoind           string `default:"testnet" required:"true" env:"network"`
		ConfirmationsNeeded int    `default:"60" required:"false"`
	}

	WebAPI struct {
		Port string `default:"8080" env:"port"`
		Bind string `default:"" env:"bind"` // optional interface IP address
	}

	Store struct {
		DBFile string `default:"gigawallet.db"`
	}

	Loggers map[string]struct {
		Path  string
		Types []string `default:"[]"`
	}

	// info for connecting to dogecoin-core daemon
	Dogecoind map[string]NodeConfig
	// currently active NodeConfig
	Core NodeConfig
}

func LoadConfig(confPath string) Config {
	c := Config{Dogecoind: make(map[string]NodeConfig)}
	configor.Load(&c, confPath)
	// config load never fails, so validate:
	if len(c.Gigawallet.Dogecoind) < 1 {
		panic("bad config: missing gigawallet.dogecoind (select active network)")
	}
	c.Core = c.Dogecoind[c.Gigawallet.Dogecoind]
	if len(c.Core.Host) < 1 {
		panic(fmt.Sprintf("bad config: missing dogecoind.%s.host", c.Gigawallet.Dogecoind))
	}
	return c
}
