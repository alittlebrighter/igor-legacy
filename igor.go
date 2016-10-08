package main

import (
	"encoding/json"
	"flag"
	"io/ioutil"
	"net/rpc"
	"os"
	"path/filepath"
	"sync"
	"time"

	log "github.com/Sirupsen/logrus"
	"github.com/alittlebrighter/switchboard-client/security"
	sModels "github.com/alittlebrighter/switchboard/models"
	"github.com/nats-io/nats"
	"github.com/radovskyb/watcher"
	uuid "github.com/satori/go.uuid"

	"github.com/alittlebrighter/igor/models"
	"github.com/alittlebrighter/igor/modules"
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

	subClient.subscription, err = conn.Subscribe(modules.ModulePrefix+moduleName, func(subj, reply string, env *sModels.Envelope) {
		data, err := security.DecryptFromString(env.Contents)
		if err != nil {
			log.WithError(err).Errorln("Could not decrypt the contents of the message.")
			return
		}

		contents := new(models.Request)
		if json.Unmarshal(data, contents); err != nil {
			log.WithError(err).Errorln("Could not unmarshal the contents of the message.")
			return
		}

		resp := new(models.Response)
		if err := subClient.client.Call(moduleName+"."+contents.Method, contents, resp); err != nil {
			log.WithError(err).Errorln("Something went wrong on the RPC server.")
			return
		}

		mData, err := json.Marshal(resp)
		if err != nil {
			log.WithError(err).Errorln("Could not marshal the contents of the response.")
			return
		}

		env.Contents, err = security.EncryptToString(mData)
		if err != nil {
			log.WithError(err).Errorln("Could not encrypt the response.")
			return
		}

		log.WithField("topic", reply).Debugln("Publishing reply.")
		conn.Publish(reply, env)
	})

	return subClient, err
}

func main() {
	configFileName := flag.String("config", "/etc/igor/igor.conf", "The JSON formatted file the specifies the configuration Igor should use.")
	debugMode := flag.Bool("debug", false, "Sets the logging level to DEBUG.")
	flag.Parse()

	log.SetLevel(log.WarnLevel)
	if *debugMode {
		log.SetLevel(log.DebugLevel)
		log.Debug("Set logging level to DebugLevel.")
	}

	configFile, err := ioutil.ReadFile(*configFileName)
	if err != nil {
		log.WithFields(log.Fields{
			"fileName": *configFileName,
			"error":    err,
		}).Fatalln("Configuration file could not be read.")
	}

	config := new(Config)
	if err := json.Unmarshal(configFile, config); err != nil {
		log.WithFields(log.Fields{
			"fileName": *configFileName,
			"error":    err,
		}).Fatalln("Configuration file could not be parsed.")
	}

	if _, err := os.Stat(config.Keyfile); os.IsNotExist(err) {
		log.WithFields(log.Fields{
			"fileName": config.Keyfile,
			"error":    err,
		}).Fatalln("Key file not found.")
	}
	security.SetSharedKeyFile(config.Keyfile)

	// setup connection to local gnatsd server
	// TODO: run through interface so we aren't specifically dependent on nats
	nc, err := nats.Connect("nats://"+config.PrivateRelay, nats.UserInfo("rpi", "tastypi314"))
	if err != nil {
		log.WithFields(log.Fields{
			"brokerHost": config.PrivateRelay,
			"error":      err,
		}).Fatalln("Could not connect to message broker.")
	}

	ec, err := nats.NewEncodedConn(nc, nats.GOB_ENCODER)
	if err != nil {
		log.WithFields(log.Fields{
			"brokerHost": config.PrivateRelay,
			"error":      err,
		}).Fatalln("Could not initiate encoded connection.")
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
					log.WithField("module", file).Debugln("New module found.")
					subClient, err := subscribeModule(ec, config.ModuleSocketDir, file)
					if err != nil {
						log.WithFields(log.Fields{
							"subscription": file,
							"error":        err,
						}).Errorln("Could not subscribe.")
					} else {
						subscriptions[file] = subClient
					}
				case watcher.EventFileDeleted:
					log.WithField("module", file).Debugln("Module closed.")
					err := subscriptions[file].client.Close()
					err = subscriptions[file].subscription.Unsubscribe()
					if log.GetLevel() == log.DebugLevel {
						log.WithError(err).Errorln("Could not close RPC client or unsubscribe.")
					}
					delete(subscriptions, file)
				}
			case err := <-w.Error:
				log.WithError(err).Errorln("ERROR: File event resulted in an error.")
				return
			}
		}
	}()

	if err := w.Add(config.ModuleSocketDir); err != nil {
		log.WithFields(log.Fields{
			"directory": config.ModuleSocketDir,
			"error":     err,
		}).Fatalln("Could not watch module socket directory.")
	}

	if err := w.Start(time.Duration(5) * time.Second); err != nil {
		log.WithError(err).Fatalln("Could not start watching module socket directory.")
	}

	wg.Add(1)

	wg.Wait()
}
