package main

import (
	"encoding/json"
	"fmt"
	"os"

	flag "github.com/spf13/pflag"

	"github.com/BurntSushi/toml"
	giga "github.com/dogecoinfoundation/gigawallet/pkg"
)

func main() {

	// building the config is a multi step process, starting with
	// default arguments, then loading config file, then applying
	// any command-line flags:

	// Default config values
	var config giga.Config = giga.Config{
		Gigawallet: giga.GigawalletConfig{
			ServiceName:         "Example Dogecoin Store",
			ServiceDomain:       "example.com",
			ServiceIconURL:      "https://example.com/icon.png",
			ServiceKeyHash:      "",
			Network:             "testnet",
			ConfirmationsNeeded: 6,
			RejectionsNeeded:    12,
		},
		WebAPI: giga.WebAPIConfig{
			AdminPort:     "8081",
			AdminBind:     "localhost",
			PubPort:       "8082",
			PubBind:       "localhost",
			PubAPIRootURL: "http://localhost:8082",
		},
		Store: giga.StoreConfig{
			DBFile: "gigawallet.db",
		},
		Loggers:   make(map[string]giga.LoggersConfig),
		Dogecoind: make(map[string]giga.NodeConfig),
		Core:      giga.NodeConfig{},
	}

	subCommandArgs := SubCommandArgs{}

	// Config file loading:
	err := mergeConfigFromFile(&config)
	if err != nil {
		fmt.Println("Failed to load config file:", err)
		os.Exit(1)
	}

	// cli flag loading
	applyFlags(&config, &subCommandArgs)

	// set config.Core to the network block specified in
	// config.Gigawallet.Network
	if len(config.Gigawallet.Network) < 1 {
		panic("bad config: missing network")
	}
	config.Core = config.Dogecoind[config.Gigawallet.Network]
	if len(config.Core.Host) < 1 {
		panic(fmt.Sprintf("bad config: missing network: %s", config.Gigawallet.Network))
	}

	// Sub commands!
	switch flag.Arg(0) {
	case "server":
		Server(config)
		os.Exit(0)
	case "printconf":
		o, _ := json.MarshalIndent(config, ">", " ")
		fmt.Println(string(o))
		os.Exit(0)
	case "setsyncheight":
		// Sets the sync block height and re-indexes the chain of a
		// running GigaWallet instance.
		if flag.Arg(1) == "" {
			fmt.Println("Provide a block height, ie: gigawallet setsyncheight 12345")
			os.Exit(0)
		}
		err := SetSyncHeight(flag.Arg(1), config, subCommandArgs)
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}
		os.Exit(0)
	default:
		fmt.Println("Invalid subcommand:", flag.Arg(0))
		os.Exit(1)
	}
}

// we search for a config.toml in /etc/gigawallet, $HOME/.gigawallet or .
// if GIGA_ENV is provided it replaces the config filename, ie: $GIGA_ENV.toml
func mergeConfigFromFile(config *giga.Config) error {

	filename := ""
	searchPaths := []string{"./", "/etc/gigawallet/", "$HOME/.gigawallet/"}

	// Check if GIGA_ENV environment variable is set and add its value as a filename
	gigaEnv, set := os.LookupEnv("GIGA_ENV")
	if set {
		filename = gigaEnv + ".toml"
	} else {
		filename = "config.toml"
	}

	// Try to find and load the config file
	for _, path := range searchPaths {
		filePath := os.ExpandEnv(path + filename)
		_, err := os.Stat(filePath)
		if err == nil {
			_, err := toml.DecodeFile(filePath, config)
			return err
		}
	}

	return fmt.Errorf("config file %s not found in %s", filename, searchPaths)
}

func applyFlags(config *giga.Config, subs *SubCommandArgs) {
	// Config file overrides
	flag.StringVar(&config.Gigawallet.ServiceName, "service-name", config.Gigawallet.ServiceName, "Service name")
	flag.StringVar(&config.Gigawallet.ServiceDomain, "service-domain", config.Gigawallet.ServiceDomain, "Service domain")
	flag.StringVar(&config.Gigawallet.ServiceIconURL, "service-icon-url", config.Gigawallet.ServiceIconURL, "Service icon URL")
	flag.StringVar(&config.Gigawallet.ServiceKeyHash, "service-key-hash", config.Gigawallet.ServiceKeyHash, "Service key hash")
	flag.StringVar(&config.Gigawallet.Network, "network", config.Gigawallet.Network, "Network")
	flag.IntVar(&config.Gigawallet.ConfirmationsNeeded, "confirmations-needed", config.Gigawallet.ConfirmationsNeeded, "Confirmations needed")
	flag.StringVar(&config.WebAPI.AdminPort, "admin-port", config.WebAPI.AdminPort, "Admin API port")
	flag.StringVar(&config.WebAPI.AdminBind, "admin-bind", config.WebAPI.AdminBind, "Admin API bind")
	flag.StringVar(&config.WebAPI.PubPort, "pub-port", config.WebAPI.PubPort, "Pub API port")
	flag.StringVar(&config.WebAPI.PubBind, "pub-bind", config.WebAPI.PubBind, "Pub API bind")
	flag.StringVar(&config.Store.DBFile, "store-db-file", config.Store.DBFile, "Store DB file")
	// Extra arguments for various subcommands
	flag.StringVar(&subs.RemoteAdminServer, "remote-admin-server", "", "http/s base URL for a remote GigaWallet server to command")
	flag.Parse()
}

type SubCommandArgs struct {
	RemoteAdminServer string
}
