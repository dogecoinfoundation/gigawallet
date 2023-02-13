package messages

import (
	"context"
	"log"

	giga "github.com/dogecoinfoundation/gigawallet/pkg"
	"gopkg.in/natefinch/lumberjack.v2"
)

type MessageLoggerConfig struct {
	types []string "default:[]"
}

type MessageLogger struct {
	// MessageLogger receives giga.Message via Rec
	Rec chan giga.Message
	// and logs them via Log
	Log *log.Logger
}

// Implements goga.MessageSubscriber
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
				stopped <- true
				return
			case msg := <-l.Rec:
				l.Log.Printf("%s (%s): %s\n",
					msg.MessageType,
					msg.ID,
					msg.Message)
			}
		}
	}()
	return nil
}

func NewMessageLogger(c MessageLoggerConfig) MessageLogger {
	// create a MessageLogger
	l := MessageLogger{
		make(chan giga.Message),
		log.New(&lumberjack.Logger{
			Filename: "./events.log",
			Compress: true,
		}, "", log.Ltime|log.Lmicroseconds),
	}
	return l
}
