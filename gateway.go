package main

import (
	"encoding/json"
	"log"
	"time"

	"github.com/alittlebrighter/switchboard-client"
	"github.com/alittlebrighter/switchboard-client/security"
	"github.com/alittlebrighter/switchboard/models"
	"github.com/nats-io/nats"
	uuid "github.com/satori/go.uuid"

	"github.com/alittlebrighter/igor/common"
)

func ConnectToWWW(config *Config, conn *nats.EncodedConn) error {
	id := config.ID
	if id == nil {
		newID := uuid.NewV1()
		id = &newID
		log.Printf("ID: %s\n", id.String())
	}

	// setup connection to public relay server
	client := relayClient.New(id, config.PublicRelay, config.Keyfile, json.Marshal, json.Unmarshal)
	if err := client.OpenSocket(); err != nil {
		log.Printf("ERROR: Could not open websocket connection to %s.  Reason: %s\n", config.PublicRelay, err.Error())
	}

	incoming, err := client.ReadMessages()
	if err != nil {
		return err
	}

	// start reading and processing incoming envelopes
	go processEnvelopes(client, incoming, conn)

	return nil
}

func processEnvelopes(client *relayClient.RelayClient, incoming chan *models.Envelope, out *nats.EncodedConn) {
	for envelope := range incoming {
		// TODO: verify the message is from an approved sender by reading the signature
		data, err := security.DecryptFromString(envelope.Contents)
		if err != nil {
			log.Printf("ERROR: Could not decrypt the contents of the message.  Reason: %s\n", err.Error())
			continue
		}

		contents := new(common.Request)
		// TODO: unmarshal from any serialization format
		if json.Unmarshal(data, contents); err != nil {
			log.Printf("ERROR: Could not unmarshal the contents of the message.  Reason: %s\n", err.Error())
			continue
		}

		response := new(models.Envelope)
		if err = out.Request(common.ModulePrefix+contents.Module, envelope, response, 2*time.Second); err != nil {
			log.Printf("ERROR: Could not solicit response from module %s.  Reason: %s\n", contents.Module, err.Error())
			continue
		}

		response.To = envelope.From
		response.From = envelope.To

		// TODO: generate signature
		response.Signature = ""

		client.SendMessage(response)
	}

	log.Println("Channel closed.  All incoming messages have been processed.")
}
