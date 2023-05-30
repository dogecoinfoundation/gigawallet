package receivers

import (
	"context"
	"fmt"
	"log"

	giga "github.com/dogecoinfoundation/gigawallet/pkg"
	"github.com/dogecoinfoundation/gigawallet/pkg/conductor"
	"gopkg.in/natefinch/lumberjack.v2"
)

type MessageLogger struct {
	// MessageLogger receives giga.Message via Rec
	Rec chan giga.Message
	// and logs them via Log
	Log *log.Logger
}

// Implements giga.MessageSubscriber
func (l MessageLogger) GetChan() chan giga.Message {
	return l.Rec
}

// Implements conductor.Service
func (l MessageLogger) Run(started, stopped chan bool, stop chan context.Context) error {
	go func() {
		started <- true
		for {
			select {
			// handle stopping the service
			case <-stop:
				close(l.Rec)
				close(stopped)
				return
			case msg := <-l.Rec:
				l.Log.Printf("%s:%s (%s): %s\n",
					msg.EventType.Type(),
					msg.EventType,
					msg.ID,
					msg.Message)
			}
		}
	}()
	return nil
}

func NewMessageLogger(path string) MessageLogger {
	// create a MessageLogger
	l := MessageLogger{
		make(chan giga.Message, 1000),
		log.New(&lumberjack.Logger{
			Filename: path,
			Compress: true,
		}, "", log.Ltime|log.Lmicroseconds),
	}
	return l
}

// Reads config and sets up any configured loggers
func SetupLoggers(cond *conductor.Conductor, bus giga.MessageBus, conf giga.Config) {
	for name, c := range conf.Loggers {
		l := NewMessageLogger(c.Path)
		cond.Service(fmt.Sprintf("Logger %s", c.Path), l)

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
				fmt.Printf("⚠️  Logger %s: ignoring invalid message type: %s\n", name, t)
			}
		}
		bus.Register(l, types...)
	}
}
