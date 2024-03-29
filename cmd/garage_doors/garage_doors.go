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
	"flag"
	"io/ioutil"
	"os"

	log "github.com/Sirupsen/logrus"

	"github.com/alittlebrighter/igor/models"
	"github.com/alittlebrighter/igor/modules/garage_doors"
)

func main() {
	configFileName := flag.String("config", "/etc/igor/modules/garage_doors.conf", "The JSON formatted file the specifies the configuration Igor should use.")
	debugMode := flag.Bool("debug", false, "Sets the logging level to DEBUG.")
	flag.Parse()

	log.SetLevel(log.WarnLevel)
	if *debugMode {
		log.SetLevel(log.DebugLevel)
		log.Debug("Logging level set to DebugLevel.")
	}

	if _, err := os.Stat(*configFileName); err != nil {
		log.WithError(err).Fatalln("Configuration file does not exist.")
	}

	configFile, err := ioutil.ReadFile(*configFileName)
	if err != nil {
		log.WithError(err).Fatalln("Configuration file could not be read.")
	}

	module := new(garageDoors.GarageDoors)
	module.Configure(models.Request{Args: configFile}, nil)
	log.Debugln("Module configured.")

	if err := garageDoors.ServeRPC(module); err != nil {
		log.WithError(err).Fatalln("Module RPC server could not be started.")
	}
}
