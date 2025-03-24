package giga

type Config struct {
	Gigawallet GigawalletConfig
	WebAPI     WebAPIConfig
	Store      StoreConfig
	Loggers    map[string]LoggersConfig
	Callbacks  map[string]CallbackConfig
	MQTT       MQTTConfig

	// Map of available networks, config.Core will be set to
	// the one specified by config.Gigawallet.Network
	Dogecoind map[string]NodeConfig
	Core      NodeConfig
}

type GigawalletConfig struct {
	// Doge Connect service domain, where is GW hosted?
	ServiceDomain string

	// Doge Connect service name, ie: Doge Payments Inc.
	ServiceName string

	// Doge Connect service icon, displayed beside name.
	ServiceIconURL string

	// A DOGENS key-hash that appears in a DOGENS DNS TXT record
	// at the ServiceDomain, will be looked up by clients to verify
	// Doge Connect messages were signed with ServiceKeySecret
	ServiceKeyHash string

	// The private key used by this GW to sign all Doge Connect
	// envelopes, consider using --service-key-secret with an
	// appropriate secret management service when deploying, rather
	// than embedding in your config file.
	ServiceKeySecret string

	// key for which Dogecoind struct to use, ie: mainnet, testnet
	Network string

	// Default number of confirmations needed to mark an invoice
	// as paid, this can be overridden per invoice using the create
	// invoice API, default 6
	ConfirmationsNeeded int

	// Default number of confirmations after a fork before an invoice
	// is marked as a double-spend and warnings are thrown. This only
	// occurs if a confirmation has already been issued. Default 6
	RejectionsNeeded int
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
	// Admin API
	AdminPort        string
	AdminBind        string // optional interface IP address
	AdminBearerToken string // optional bearer token for authenticating admin API requests

	// Public API
	PubPort       string
	PubBind       string // optional interface IP address
	PubAPIRootURL string // ie: https://example.com/gigawallet
}

type StoreConfig struct {
	DBFile string
}

type LoggersConfig struct {
	Path  string
	Types []string
}

type CallbackConfig struct {
	Path  string
	Types []string
}

type MQTTConfig struct {
	Address  string
	Username string
	Password string
	ClientID string
	Queues   map[string]MQTTQueueConfig
}

type MQTTQueueConfig struct {
	TopicFilter string
	Types       []string
}

func TestConfig() Config {
	return Config{
		Gigawallet: GigawalletConfig{
			ServiceName:         "Example Dogecoin Store",
			ServiceDomain:       "example.com",
			ServiceIconURL:      "https://example.com/icon.png",
			ServiceKeyHash:      "",
			Network:             "testnet",
			ConfirmationsNeeded: 6,
			RejectionsNeeded:    12,
		},
		WebAPI: WebAPIConfig{
			AdminPort:     "8081",
			AdminBind:     "localhost",
			PubPort:       "8082",
			PubBind:       "localhost",
			PubAPIRootURL: "http://localhost:8082",
		},
		Store: StoreConfig{
			DBFile: ":memory:",
		},
		MQTT: MQTTConfig{
			Queues: make(map[string]MQTTQueueConfig),
		},
		Loggers:   make(map[string]LoggersConfig),
		Dogecoind: make(map[string]NodeConfig),
		Core:      NodeConfig{},
	}
}
