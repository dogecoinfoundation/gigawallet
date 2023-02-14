package conductor

import (
	"os"
	"os/signal"
	"syscall"
	"time"
)

// Sets the time allowed for a service to start before timing out
func StartupTimeout(d time.Duration) func(*Conductor) {
	return func(c *Conductor) {
		c.startTimeout = d
	}
}

// Sets the time allowed for a service to stop before timing out
func ShutdownTimeout(d time.Duration) func(*Conductor) {
	return func(c *Conductor) {
		c.stopTimeout = d
	}
}

// tells the Conductor to log output
func Noisy() func(*Conductor) {
	return func(c *Conductor) {
		c.noisy = true
	}
}

// This hooks SIGTERM and SIGINT and will shut down the Conductor
// if one is detected.
func HookSignals() func(*Conductor) {
	return func(c *Conductor) {
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, syscall.SIGTERM, syscall.SIGINT)
		go func() {
			for {
				select {
				case sig := <-sigCh: // sigterm/sigint caught
					c.logf("Caught %v signal, shutting down", sig)
					c.Stop()
					continue
				case <-c.shutdown: // service is closing down..
					return
				}
			}
		}()
	}
}
