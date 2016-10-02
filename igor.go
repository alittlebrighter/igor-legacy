package main

import (
	"encoding/json"
	"flag"
	"io/ioutil"
	"log"
	"net/rpc"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/alittlebrighter/switchboard-client/security"
	"github.com/alittlebrighter/switchboard/models"
	"github.com/nats-io/nats"
	"github.com/radovskyb/watcher"
	uuid "github.com/satori/go.uuid"

	"github.com/alittlebrighter/igor/common"
)

type Config struct {
	ID                                                  *uuid.UUID
	PublicRelay, PrivateRelay, Keyfile, ModuleSocketDir string
}

type subscriptionClient struct {
	subscription *nats.Subscription
	client       *rpc.Client
}

func subscribeModule(conn *nats.EncodedConn, socketDir, moduleName string) (*subscriptionClient, error) {
	subClient := new(subscriptionClient)
	var err error
	subClient.client, err = rpc.Dial("unix", socketDir+moduleName)
	if err != nil {
		return nil, err
	}

	subClient.subscription, err = conn.Subscribe(common.ModulePrefix+moduleName, func(subj, reply string, env *models.Envelope) {
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

		resp := new(common.Response)
		if err := subClient.client.Call(moduleName+"."+contents.Method, contents, resp); err != nil {
			log.Printf("ERROR: Something went wrong on the RPC server: %s\n", err.Error())
			return
		}

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

		conn.Publish(reply, env)
	})

	return subClient, err
}

func main() {
	configFileName := flag.String("config", "/etc/igor/igor.conf", "The JSON formatted file the specifies the configuration Igor should use.")
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

	ec, err := nats.NewEncodedConn(nc, nats.GOB_ENCODER)
	if err != nil {
		log.Fatalf("ERROR: Could not create encoded connection: %s\n", err.Error())
	}

	ConnectToWWW(config, ec)

	subscriptions := map[string]*subscriptionClient{}
	defer func() {
		for key, sub := range subscriptions {
			sub.client.Close()
			sub.subscription.Unsubscribe()
			delete(subscriptions, key)
		}
	}()

	wg := new(sync.WaitGroup)

	w := watcher.New()

	go func() {
		defer wg.Done()
		for {
			select {
			case event := <-w.Event:
				file := filepath.Base(event.Name())

				// Print out the file name with a message
				// based on the event type.
				switch event.EventType {
				case watcher.EventFileAdded:
					subClient, err := subscribeModule(ec, config.ModuleSocketDir, file)
					if err != nil {
						log.Printf("ERROR: Could not subscribe to %s. Reason: %s\n", file, err.Error())
					} else {
						subscriptions[file] = subClient
					}
				case watcher.EventFileDeleted:
					subscriptions[file].client.Close()
					subscriptions[file].subscription.Unsubscribe()
					delete(subscriptions, file)
				}
			case err := <-w.Error:
				log.Printf("ERROR: File event resulted in an error: %s\n", err.Error())
				return
			}
		}
	}()

	if err := w.Add(config.ModuleSocketDir); err != nil {
		log.Fatalln(err)
	}

	w.Start(time.Duration(5) * time.Second)

	wg.Add(1)

	wg.Wait()
}
