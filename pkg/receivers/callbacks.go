package receivers

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	giga "github.com/dogecoinfoundation/gigawallet/pkg"
	"github.com/dogecoinfoundation/gigawallet/pkg/conductor"
)

func NewCallbackSender(path string, bus giga.MessageBus) CallbackSender {
	// create a MessageLogger
	return CallbackSender{
		make(chan giga.Message, 1000),
		path,
		bus,
	}
}

type CallbackSender struct {
	// incomming msgs
	Rec  chan giga.Message
	Path string
	Bus  giga.MessageBus
}

// Implements giga.MessageSubscriber
func (s CallbackSender) GetChan() chan giga.Message {
	return s.Rec
}

// Implements conductor.Service
func (s CallbackSender) Run(started, stopped chan bool, stop chan context.Context) error {
	go func() {
		started <- true
		for {
			select {
			// handle stopping the service
			case <-stop:
				close(s.Rec)
				close(stopped)
				return
			case msg := <-s.Rec:
				s.Bus.Send(giga.SYS_MSG, fmt.Sprintf("CallbackSender: Sending msg %s: %v", s.Path, msg))
				err := postWithRetry(s.Path, msg, s.Bus)
				if err != nil {
					s.Bus.Send(giga.SYS_ERR, fmt.Sprintf("CallbackSender: %v", msg))
				}
			}
		}
	}()
	return nil
}

// Reads config and sets up any configured callbacks
func SetupCallbacks(cond *conductor.Conductor, bus giga.MessageBus, conf giga.Config) {
	for name, c := range conf.Callbacks {
		s := NewCallbackSender(c.Path, bus)
		cond.Service(fmt.Sprintf("Callback sender for: %s", c.Path), s)

		types := []giga.EventType{}
		for _, t := range c.Types {
			match := false
			for _, x := range giga.EVENT_TYPES {
				if t == x.Type() {
					match = true
					types = append(types, x)
				}
			}
			if !match {
				fmt.Printf("⚠️  Callback %s: ignoring invalid message type: %s\n", name, t)
			}
		}
		bus.Register(s, types...)
	}
}

func postWithRetry(path string, obj interface{}, bus giga.MessageBus) error {
	maxRetries := 6
	initialDelay := 1 * time.Second
	maxDelay := 32 * time.Second

	objJSON, err := json.Marshal(obj)
	if err != nil {
		bus.Send(giga.SYS_ERR, fmt.Sprintf("CallbackSender: Failed to serialize object to JSON: %v", err))
		return err
	}

	req, err := http.NewRequest("POST", path, bytes.NewBuffer(objJSON))
	if err != nil {
		bus.Send(giga.SYS_ERR, fmt.Sprintf("CallbackSender: Failed to create request: %v", err))
		return err
	}

	client := &http.Client{Timeout: 30 * time.Second}

	go func() {
		retryCount := 0
		delay := initialDelay

		for retryCount <= maxRetries {
			resp, err := client.Do(req)
			if err == nil && resp.StatusCode == 200 {
				// Successful request
				bus.Send(giga.SYS_MSG, fmt.Sprintf("CallbackSender: success! %s", path))
				resp.Body.Close()
				return
			}

			bus.Send(giga.SYS_MSG, fmt.Sprintf("CallbackSender: Request failed (attempt %d/%d). Retrying in %v. Error: %v", retryCount+1, maxRetries+1, delay, err))
			time.Sleep(delay)

			// Increase delay exponentially, with a maximum limit
			delay *= 2
			if delay > maxDelay {
				delay = maxDelay
			}

			retryCount++
		}

		bus.Send(giga.SYS_ERR, fmt.Sprintf("CallbackSender: Request failed after maximum retries. Aborting: %s", path))
	}()

	return nil
}
