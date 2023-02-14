package conductor

import (
	"context"
	"fmt"
	"sync"
	"time"
)

const (
	startupTimeout  time.Duration = time.Duration(5 * time.Second)
	shutdownTimeout time.Duration = time.Duration(5 * time.Second)
)

type Service interface {
	Run(chan bool, chan bool, chan context.Context) error
}

type serviceState struct {
	name     string
	service  Service
	ready    chan bool
	stopped  chan bool
	shutdown chan context.Context
}

type Conductor struct {
	started      bool          // Have we been started yet?
	noisy        bool          // Should we log?
	startTimeout time.Duration // How long should we wait for each service to start before we die?
	stopTimeout  time.Duration // How long should we wait for each service to stop before we kill it?
	shutdown     chan bool     // channel to block on, indicates everything has stopped, returned from Start()
	services     []*serviceState
}

/* Create a new conductor instance, accepts Option funcs (see README.md)
for changing default behaviours */
func NewConductor(opts ...func(*Conductor)) *Conductor {
	c := Conductor{
		started:      false,
		noisy:        false,
		startTimeout: startupTimeout,
		stopTimeout:  shutdownTimeout,
		shutdown:     make(chan bool),
		services:     []*serviceState{},
	}

	for _, optFn := range opts {
		optFn(&c)
	}
	return &c
}

/* Add a Service with a name to be started in order when Start is called */
func (c *Conductor) Service(name string, service Service) {
	if c.started {
		panic("Cannot call Conductor.Service after Conductor.Start")
	}
	c.services = append(c.services,
		&serviceState{name, service, make(chan bool, 1), make(chan bool, 1), make(chan context.Context, 1)})
}

/* Start the conductor, each service is started in turn */
func (c *Conductor) Start() chan bool {
	c.started = true

	// start each ManagedService one at a time, this gives us service dependency order.
SRV_LOOP:
	for _, srv := range c.services {
		c.logf("🔧 Starting '%s':\n", srv.name)
		err := srv.service.Run(srv.ready, srv.stopped, srv.shutdown)
		if err != nil {
			// Service has failed to start with an error, shutdown everything
			c.logf("⚠️  '%s' exited with: %s\n", srv.name, err)
			c.Stop()
			break
		}
		select {
		case <-time.After(c.startTimeout):
			// Service has timed out, shutdown everything
			c.logf("⚠️  timed-out during startup %s\n", srv.name)
			c.Stop()
			break SRV_LOOP
		case <-srv.ready:
			// Service started up ok!
			c.logf(".. ok\n")
			continue
		}
	}
	return c.shutdown
}

// stop the conductor, begin shutting down services
func (c *Conductor) Stop() {
	// signal all services they should shutdown within timeout seconds
	ctx, _ := context.WithTimeout(context.Background(), c.stopTimeout)

	wg := sync.WaitGroup{}
	// we're waiting for this many services to close..
	wg.Add(len(c.services))

	// create a done channel that gets closed when all services are shutdown
	done := make(chan bool)
	go func() {
		wg.Wait()
		done <- true
	}()

	// decrement our waitgroup when each service says it has stopped
	for _, state := range c.services {
		fmt.Println("Requesting shutdown: ", state.name)
		state.shutdown <- ctx
		go func(s *serviceState) {
			<-(*s).stopped
			fmt.Println("Shutdown complete: ", (*s).name)
			wg.Done()
		}(state)
	}

	// Wait for either all services to close, or the timeout to occur then signal shutdown.
	select {
	case <-done:
		fmt.Println("👋 All services stopped, goodbye!")
		close(c.shutdown)
		return
	case <-time.After(c.stopTimeout + time.Second):
		fmt.Println("Timeout exeeded waiting for services to stop, shutting down")
		close(c.shutdown)
	}
}

func (c *Conductor) logf(s string, v ...interface{}) {
	if c.noisy {
		fmt.Printf(s, v...)
	}
}

func (c *Conductor) log(v ...interface{}) {
	if c.noisy {
		fmt.Print(v...)
	}
}
