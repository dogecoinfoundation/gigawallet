package receivers

import (
	giga "github.com/dogecoinfoundation/gigawallet/pkg"
	"github.com/dogecoinfoundation/gigawallet/pkg/conductor"
)

// Sets up standard receivers.
func SetUpReceivers(cond *conductor.Conductor, bus giga.MessageBus, conf giga.Config) {
	// Set up configured loggers
	SetupLoggers(cond, bus, conf)

	// Set up configured Callbacks
	SetupCallbacks(cond, bus, conf)

	// Set up configured MQTT queues
	SetupMQTTs(cond, bus, conf)
}
