package giga

type Config struct {
	Gigawallet GigawalletConfig
	WebAPI     WebAPIConfig
	Store      StoreConfig
	Loggers    map[string]LoggersConfig

	// Map of available networks, config.Core will be set to
	// the one specified by config.Gigawallet.Network
	Dogecoind map[string]NodeConfig
	Core      NodeConfig
}

type GigawalletConfig struct {
	// Keys used by Doge Connect to identify and verify this
	// Gigawallet instance.
	ServiceName string
	// ServiceDomain will be used to lookup the ServiceKey from
	// DNS TXT record and verified against ServiceKeyHash.
	ServiceDomain  string
	ServiceIconURL string
	ServiceKeyHash string

	// key for which Dogecoind struct to use
	Network             string
	ConfirmationsNeeded int
}

type NodeConfig struct {
	Host    string
	ZMQPort int
	RPCHost string
	RPCPort int
	RPCPass string
	RPCUser string
}

type WebAPIConfig struct {
	Port string
	Bind string // optional interface IP address
}

type StoreConfig struct {
	DBFile string
}

type LoggersConfig struct {
	Path  string
	Types []string
}
