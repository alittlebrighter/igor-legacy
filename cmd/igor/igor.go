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
package main

import (
	"encoding/json"
	"flag"
	"io/ioutil"
	"os"
	"sync"
	"time"

	log "github.com/Sirupsen/logrus"
	"github.com/alittlebrighter/switchboard-client/security"
	"github.com/nats-io/nats"
	"github.com/radovskyb/watcher"

	"github.com/alittlebrighter/igor"
)

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

	config := new(igor.Config)
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

	igor.ConnectToWWW(config, ec)

	subscriptions := map[string]*igor.SubscriptionClient{}
	defer func() {
		for key, sub := range subscriptions {
			sub.Client.Close()
			sub.Subscription.Unsubscribe()
			delete(subscriptions, key)
		}
	}()

	wg := new(sync.WaitGroup)

	w := watcher.New()

	go func() {
		defer wg.Done()
		igor.ProcessFileEvents(w, config.ModuleSocketDir, subscriptions, ec)
	}()

	if err := w.Add(config.ModuleSocketDir); err != nil {
		log.WithFields(log.Fields{
			"directory": config.ModuleSocketDir,
			"error":     err,
		}).Fatalln("Could not add module socket directory to watch list.")
	}

	log.WithField("directory", config.ModuleSocketDir).Debugln("Starting file watcher.")
	if err := w.Start(time.Duration(5) * time.Second); err != nil {
		log.WithError(err).Fatalln("Could not start watching module socket directory.")
	}

	wg.Add(1)

	wg.Wait()
}
