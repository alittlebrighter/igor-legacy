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
package garageDoors

import (
	"encoding/json"
	"net"
	"net/rpc"
	"os"
	"time"

	log "github.com/Sirupsen/logrus"

	"github.com/alittlebrighter/igor/models"
	"github.com/alittlebrighter/igor/modules"
)

type Config struct {
	modules.BaseConfig
	Pins                          map[string]int
	TriggerTime, ForceTriggerTime time.Duration
}

type GarageDoors struct {
	modules.BaseModule
	doors map[string]*GarageDoorController
}

func (gd *GarageDoors) Configure(req models.Request, response *models.Response) error {
	config := new(Config)
	if err := json.Unmarshal(req.Args, config); err != nil {
		response.Success = false
		response.Data["message"] = "Error parsing arguments."
		log.Errorln("Could not unmarshal arguments.")
		return nil
	}

	gd.Name = config.Name
	gd.SocketDir = config.SocketDir
	gd.doors = make(map[string]*GarageDoorController)
	for label, pin := range config.Pins {
		controller, err := NewGarageDoorController(pin, config.TriggerTime, config.ForceTriggerTime)
		if err != nil {
			return err
		}
		gd.doors[label] = controller
	}

	return nil
}

const triggerDoc = `{
    "human": "Trigger triggers a garage door normally or forced (trigger lasts until door is completely open or closed).",
    "methodName": "trigger",
    "args": [
        {"name": "door", "type": "string", "options": %v, "required": true},
        {"name": "force", "type": "boolean", "required": false}
    ]
}`

func (gd *GarageDoors) Docs(req models.Request, response *models.Response) error {
	response = models.NewResponse(gd.Name)
	response.Success = true
	response.Data["documentation"] = triggerDoc

	return nil
}

func (gd *GarageDoors) Trigger(req models.Request, response *models.Response) error {
	log.Debugln("Trigger called.")

	response = models.NewResponse(gd.Name)

	args := new(models.TriggerArgs)
	if err := json.Unmarshal(req.Args, args); err != nil {
		response.Success = false
		response.Data["message"] = "Error parsing arguments."
		log.Errorln("Could not unmarshal arguments.")
		return nil
	}

	if args.Door == "" {
		response.Success = false
		response.Data["message"] = "You must specify a door to trigger."
		log.Errorln("Door not specified.")
		return nil
	}

	if err := gd.doors[args.Door].Trigger(args.Force); err != nil {
		response.Success = false
		response.Data["door"] = args.Door
		response.Data["force"] = args.Force
		response.Data["message"] = "ERROR: " + err.Error()
		log.WithFields(response.Data).Errorln("Could not trigger door.")
		return nil
	}

	response.Success = true
	response.Data["door"] = args.Door
	response.Data["force"] = args.Force
	response.Data["message"] = "Garage door successfully triggered."
	log.WithFields(response.Data).Debugln("Door successfully triggered.")

	return nil
}

func ServeRPC(m *GarageDoors) error {
	// remove any previous sockets
	os.Remove(m.SocketDir + m.Name)

	listener, err := net.Listen("unix", m.SocketDir+m.Name)
	if err != nil {
		return err
	}
	defer os.Remove(m.SocketDir + m.Name)

	os.Chmod(m.SocketDir+m.Name, 0666)

	server := rpc.NewServer()
	server.RegisterName(m.Name, m)
	server.Accept(listener)
	return nil
}
