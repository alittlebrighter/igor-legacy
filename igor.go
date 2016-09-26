package main

import (
	"encoding/json"
	"flag"
	"io/ioutil"
	"log"
	"os"
	"os/signal"
	"sync"

	"github.com/alittlebrighter/switchboard-client/security"
	"github.com/alittlebrighter/switchboard/models"
	"github.com/nats-io/nats"

	"github.com/alittlebrighter/igor/common"
	"github.com/alittlebrighter/igor/modules"
)

type Config struct {
	PrivateRelay, Keyfile string
	Modules               []struct {
		Name   string
		Config map[string]interface{}
	}
}

func main() {
	configFileName := flag.String("config", "/etc/igor.conf", "The JSON formatted file the specifies the configuration Igor should use.")
	flag.Parse()

	configFile, err := ioutil.ReadFile(*configFileName)
	if err != nil {
		log.Fatalf("ERROR: Configuration file could not be read: %s\n", err.Error())
	}

	config := new(Config)
	if err := json.Unmarshal(configFile, config); err != nil {
		log.Fatalf("ERROR: Configuration file could not be parsed: %s\n", err.Error())
	}

	if _, err := os.Stat(config.Keyfile); os.IsNotExist(err) {
		log.Fatalf("ERROR: Key file not found: %s\n", err.Error())
	}
	security.SetSharedKeyFile(config.Keyfile)

	// setup connection to local gnatsd server
	// TODO: run through interface so we aren't specifically dependent on nats
	nc, err := nats.Connect("nats://"+(config.PrivateRelay), nats.UserInfo("rpi", "tastypi314"))
	if err != nil {
		log.Fatalf("ERROR: Could not connect to message broker: %s\n", err.Error())
	}
	common.SetPrivateRelayConn(nc)
	defer common.PrivateRelayConn().Close()

	ec, err := nats.NewEncodedConn(common.PrivateRelayConn(), nats.GOB_ENCODER)
	if err != nil {
		log.Fatalf("ERROR: Could not create encoded connection: %s\n", err.Error())
	}
	common.SetPrivateRelayEncConn(ec)
	defer common.PrivateRelayEncConn().Close()

	wg := new(sync.WaitGroup)

	// TODO: hack, do this cleaner
	modRequests := []chan *common.Request{}
	fromOS := make(chan os.Signal, 1)
	signal.Notify(fromOS, os.Interrupt)
	go func() {
		for range fromOS {
			for _, reqChan := range modRequests {
				close(reqChan)
				wg.Done()
			}
		}
		return
	}()

	for _, module := range config.Modules {
		run, ok := modules.Loaded[module.Name]
		if !ok {
			continue
		}

		c := common.PrivateRelayEncConn()

		requests := make(chan *common.Request)
		modRequests = append(modRequests, requests)

		responses, err := run(module.Config, requests)
		if err != nil {
			log.Printf("ERROR: %s module configuration: %s\n", module.Name, err.Error())
			continue
		}

		wg.Add(1)

		sub, err := c.Subscribe(common.ModulePrefix+module.Name, func(subj, reply string, env *models.Envelope) {
			data, err := security.DecryptFromString(env.Contents)
			if err != nil {
				log.Printf("ERROR: Could not decrypt the contents of the message.  Reason: %s\n", err.Error())
				return
			}

			contents := new(common.Request)
			// TODO: unmarshal from any serialization format
			if json.Unmarshal(data, contents); err != nil {
				log.Printf("ERROR: Could not unmarshal the contents of the message.  Reason: %s\n", err.Error())
				return
			}

			requests <- contents
			resp := <-responses

			mData, err := json.Marshal(resp)
			if err != nil {
				log.Printf("ERROR: Could not marshal the contents of the response.  Reason: %s\n", err.Error())
				return
			}

			env.Contents, err = security.EncryptToString(mData)
			if err != nil {
				log.Printf("ERROR: Could not encrypt the response.  Reason: %s\n", err.Error())
				return
			}

			c.Publish(reply, env)
		})

		if err != nil {
			log.Printf("ERROR: Could not subscribe to topic %s.\n", common.ModulePrefix+module.Name)
			continue
		}
		defer sub.Unsubscribe()
	}

	wg.Wait()
}
