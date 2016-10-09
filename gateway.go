/*
Igor, a home automation solution
Copyright (C) 2016  Adam Bright

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
GNU General Public License for more details.

You should have received a copy of the GNU General Public License
along with this program.  If not, see <http://www.gnu.org/licenses/>.
*/
package igor

import (
	"encoding/json"
	"time"

	log "github.com/Sirupsen/logrus"
	"github.com/alittlebrighter/switchboard-client"
	"github.com/alittlebrighter/switchboard-client/security"
	sModels "github.com/alittlebrighter/switchboard/models"
	"github.com/nats-io/nats"
	uuid "github.com/satori/go.uuid"

	"github.com/alittlebrighter/igor/models"
	"github.com/alittlebrighter/igor/modules"
)

func ConnectToWWW(config *Config, conn *nats.EncodedConn) error {
	log.Debugln("Connecting to public switchboard server.")

	id := config.ID
	if id == nil {
		newID := uuid.NewV1()
		id = &newID
		log.WithField("ID", id.String()).Debugln("Created new ID.")
	}

	// setup connection to public relay server
	client := relayClient.New(id, config.PublicRelay, config.Keyfile, json.Marshal, json.Unmarshal)
	if err := client.OpenSocket(); err != nil {
		return err
	}

	incoming, err := client.ReadMessages()
	if err != nil {
		return err
	}

	// start reading and processing incoming envelopes
	go processEnvelopes(client, incoming, conn)

	return nil
}

func processEnvelopes(client *relayClient.RelayClient, incoming chan *sModels.Envelope, out *nats.EncodedConn) {
	for envelope := range incoming {
		// TODO: verify the message is from an approved sender by reading the signature
		data, err := security.DecryptFromString(envelope.Contents)
		if err != nil {
			log.WithError(err).Errorln("Could not decrypt the contents of the message.")
			continue
		}

		contents := new(models.Request)
		// TODO: unmarshal from any serialization format
		if json.Unmarshal(data, contents); err != nil {
			log.WithError(err).Errorln("Could not unmarshal the contents of the message.")
			continue
		}

		log.WithFields(log.Fields{
			"topic": modules.ModulePrefix + contents.Module,
		}).Debugln("Sending request.")

		response := new(sModels.Envelope)
		if err = out.Request(modules.ModulePrefix+contents.Module, envelope, response, 2*time.Second); err != nil {
			log.WithFields(log.Fields{
				"module": contents.Module,
				"error":  err,
			}).Errorln("Could not solicit response from module.")
			continue
		}

		log.WithFields(log.Fields{
			"topic": modules.ModulePrefix + contents.Module,
		}).Debugln("Received response.")

		response.To = envelope.From
		response.From = envelope.To

		// TODO: generate signature
		response.Signature = ""

		client.SendMessage(response)
		log.WithField("requestor", response.To).Debugln("Response sent back to requestor.")
	}

	log.Warningln("Channel closed.  All incoming messages have been processed.")
}
