package receivers

import (
	"context"
	"encoding/json"
	"fmt"

	giga "github.com/dogecoinfoundation/gigawallet/pkg"
	"github.com/dogecoinfoundation/gigawallet/pkg/conductor"
	"github.com/yosssi/gmq/mqtt"
	"github.com/yosssi/gmq/mqtt/client"
)

func NewMQTTSender(config giga.MQTTConfig, bus giga.MessageBus) MQTTSender {
	return MQTTSender{
		make(chan giga.Message, 1000),
		config,
		bus,
	}
}

type MQTTSender struct {
	// incomming msgs
	Rec    chan giga.Message
	Config giga.MQTTConfig
	Bus    giga.MessageBus
}

// Implements giga.MessageSubscriber
func (s MQTTSender) GetChan() chan giga.Message {
	return s.Rec
}

// Implements conductor.Service
func (s MQTTSender) Run(started, stopped chan bool, stop chan context.Context) error {
	go func() {
		cli := client.New(&client.Options{
			// Define the processing of the error handler.
			ErrorHandler: func(err error) {
				s.Bus.Send(giga.SYS_ERR, fmt.Sprintf("MQTTSender: %s", err))
			},
		})

		// connect to MQTT Bus
		err := cli.Connect(&client.ConnectOptions{
			Network:  "tcp",
			Address:  s.Config.Address,
			ClientID: []byte(s.Config.ClientID),
			UserName: []byte(s.Config.Username),
			Password: []byte(s.Config.Password),
		})
		if err != nil {
			s.Bus.Send(giga.SYS_ERR, fmt.Sprintf("MQTTSender connection failure %s", err))
			close(s.Rec)
			close(stopped)
			return
		}

		// Successfully started up
		started <- true

		for {
			select {
			// handle stopping the service
			case <-stop:
				close(s.Rec)
				close(stopped)
				return
			case msg := <-s.Rec:

				jsonMsg, err := json.Marshal(msg)
				if err != nil {
					s.Bus.Send(giga.SYS_ERR, fmt.Sprintf("MQTTSender failed to marshall msg: %v", msg))
					break
				}

			skip:
				for _, queue := range s.Config.Queues {
					cont := false
					// check if this message is for this queue
					for _, t := range queue.Types {
						if t == "ALL" {
							cont = true
							break
						}
						if t == "SYS" {
							// We don't accept SYS msgs (to avoid loops on err)
							break skip
						}
						if t == msg.EventType.Type() {
							cont = true
						}
					}
					if !cont {
						break
					}

					//send message to topic

					err = cli.Publish(&client.PublishOptions{
						QoS:       mqtt.QoS0,
						TopicName: []byte(queue.TopicFilter),
						Message:   []byte(jsonMsg),
					})
					if err != nil {
						s.Bus.Send(giga.SYS_ERR, fmt.Sprintf("MQTTSender: %v", msg))
					}
				}
			}
		}
	}()
	return nil
}

func SetupMQTTs(cond *conductor.Conductor, bus giga.MessageBus, conf giga.Config) {
	if conf.MQTT.Address != "" {
		s := NewMQTTSender(conf.MQTT, bus)
		cond.Service("MQTT sender ", s)
		// Sub to 'ALL' because we're filtering on our side
		bus.Register(s, []giga.EventType{giga.EVENT_ALL("ALL")}...)
	}
}
