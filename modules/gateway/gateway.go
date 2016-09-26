package gateway

import (
	"encoding/json"
	"log"
	"time"

	"github.com/alittlebrighter/switchboard-client"
	"github.com/alittlebrighter/switchboard-client/security"
	"github.com/alittlebrighter/switchboard/models"
	"github.com/nats-io/nats"
	uuid "github.com/satori/go.uuid"

	modules "github.com/alittlebrighter/igor/common"
)

const Name = "gateway"

type Config struct {
	ID          *uuid.UUID
	PublicRelay string
}

func parseConfig(configMap map[string]interface{}) *Config {
	config := new(Config)
	id := uuid.FromStringOrNil(configMap["id"].(string))
	config.ID = &id
	config.PublicRelay, _ = configMap["publicRelay"].(string)
	return config
}

func Run(config map[string]interface{}, commands chan *modules.Request) (chan *modules.Response, error) {
	conf := parseConfig(config)

	id := conf.ID
	if id == nil {
		newID := uuid.NewV1()
		id = &newID
		log.Printf("ID: %s\n", id.String())
	}

	// setup connection to public relay server
	client := relayClient.New(id, conf.PublicRelay, "shared.key", json.Marshal, json.Unmarshal)
	if err := client.OpenSocket(); err != nil {
		log.Printf("ERROR: Could not open websocket connection to %s.  Reason: %s\n", conf.PublicRelay, err.Error())
	}

	incoming, err := client.ReadMessages()
	if err != nil {
		panic(err)
	}

	// start reading and processing incoming envelopes
	go processEnvelopes(client, incoming, modules.PrivateRelayEncConn())

	return make(chan *modules.Response), nil
}

func processEnvelopes(client *relayClient.RelayClient, incoming chan *models.Envelope, out *nats.EncodedConn) {
	for envelope := range incoming {
		// TODO: verify the message is from an approved sender by reading the signature
		data, err := security.DecryptFromString(envelope.Contents)
		if err != nil {
			log.Printf("ERROR: Could not decrypt the contents of the message.  Reason: %s\n", err.Error())
			continue
		}

		contents := new(modules.Request)
		// TODO: unmarshal from any serialization format
		if json.Unmarshal(data, contents); err != nil {
			log.Printf("ERROR: Could not unmarshal the contents of the message.  Reason: %s\n", err.Error())
			continue
		}

		response := new(models.Envelope)
		if err = out.Request(modules.ModulePrefix+contents.Module, envelope, response, 2*time.Second); err != nil {
			log.Printf("ERROR: Could not solicit response from module %s.  Reason: %s\n", contents.Module, err.Error())
			continue
		}

		response.To = envelope.From
		response.From = envelope.To
		client.SendMessage(response)
	}

	log.Println("Channel closed.  All incoming messages have been processed.")
}
