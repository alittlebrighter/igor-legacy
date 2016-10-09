package igor

import (
	"encoding/json"
	"net/rpc"
	"os"
	"path/filepath"

	logger "github.com/Sirupsen/logrus"
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

type SubscriptionClient struct {
	Subscription *nats.Subscription
	Client       *rpc.Client
}

func SubscribeModule(conn *nats.EncodedConn, socketDir, moduleName string) (*SubscriptionClient, error) {
	log := logger.WithField("func", "SubscribeModule")

	subClient := new(SubscriptionClient)
	var err error
	subClient.Client, err = rpc.Dial("unix", socketDir+moduleName)
	if err != nil {
		return nil, err
	}

	log.WithField("topic", modules.ModulePrefix+moduleName).Debugln("Subscribing to topic.")
	subClient.Subscription, err = conn.Subscribe(modules.ModulePrefix+moduleName, func(subj, reply string, env *sModels.Envelope) {
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
		log.WithField("RPCCall", moduleName+"."+contents.Method).Debugln("Making RPC call to module.")
		if err := subClient.Client.Call(moduleName+"."+contents.Method, contents, resp); err != nil {
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

func ProcessFileEvents(w *watcher.Watcher, watched string, subscriptions map[string]*SubscriptionClient, ec *nats.EncodedConn) {
	log := logger.WithField("func", "ProcessFileEvents")

	addSubscription := func(socket string) {
		subClient, err := SubscribeModule(ec, watched, socket)
		if err != nil {
			log.WithFields(logger.Fields{
				"subscription": socket,
				"error":        err,
			}).Errorln("Could not subscribe.")
		} else {
			subscriptions[socket] = subClient
		}
	}

	log.WithField("directory", watched).Debugln("Getting existing connections.")
	filepath.Walk(watched, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			log.WithFields(logger.Fields{
				"path":  path,
				"error": err,
			}).Debugln("Could not walk path.")
			return err
		} else if path == watched {
			return nil
		}

		addSubscription(filepath.Base(path))
		return nil
	})

	log.WithField("directory", watched).Debugln("Watching for new connections.")
	for {
		select {
		case event := <-w.Event:
			file := filepath.Base(event.Name())

			// Print out the file name with a message
			// based on the event type.
			switch event.EventType {
			case watcher.EventFileAdded:
				log.WithField("module", file).Debugln("New module found.")
				addSubscription(file)
			case watcher.EventFileDeleted:
				log.WithField("module", file).Debugln("Module closed.")
				err := subscriptions[file].Client.Close()
				err = subscriptions[file].Subscription.Unsubscribe()
				if logger.GetLevel() == logger.DebugLevel {
					log.WithError(err).Errorln("Could not close RPC client or unsubscribe.")
				}
				delete(subscriptions, file)
			}
		case err := <-w.Error:
			log.WithError(err).Errorln("File event resulted in an error.")
			return
		}
	}

	log.WithField("directory", watched).Warnln("Stopped watching directory.")
}
