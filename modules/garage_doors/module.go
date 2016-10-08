package main

import (
	"encoding/json"
	"flag"
	"io/ioutil"
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

func (gd *GarageDoors) configureModule(config *Config) error {
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

func serve(m *GarageDoors) error {
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

func main() {
	configFileName := flag.String("config", "/etc/igor/modules/garage_doors.conf", "The JSON formatted file the specifies the configuration Igor should use.")
	flag.Parse()

	if _, err := os.Stat(*configFileName); err != nil {
		log.WithError(err).Fatalln("Configuration file does not exist.")
	}

	configFile, err := ioutil.ReadFile(*configFileName)
	if err != nil {
		log.WithError(err).Fatalln("Configuration file could not be read.")
	}

	config := new(Config)
	if err := json.Unmarshal(configFile, config); err != nil {
		log.WithError(err).Fatalln("Configuration file could not be parsed.")
	}

	module := new(GarageDoors)
	module.configureModule(config)
	log.WithField("config", config).Debugln("Module configured.")

	if err := serve(module); err != nil {
		log.WithError(err).Fatalln("Module RPC server could not be started.")
	}
}
