package main

import (
	"encoding/json"
	"flag"
	"io/ioutil"
	"log"
	"net"
	"net/rpc"
	"os"
	"time"

	"github.com/alittlebrighter/igor/common"
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

func (gd *GarageDoors) Docs(req common.Request, response *common.Response) error {
	response = common.NewResponse(gd.Name)
	response.Success = true
	response.Data["documentation"] = triggerDoc

	return nil
}

func (gd *GarageDoors) Trigger(req common.Request, response *common.Response) error {
	response = common.NewResponse(gd.Name)

	reqDoor, ok := req.Args["door"]
	if !ok {
		response.Success = false
		response.Data["message"] = "You must specify a door to trigger."
		return nil
	}
	door := reqDoor.(string)

	var force bool
	if reqForce, ok := req.Args["force"]; ok {
		force = reqForce.(bool)
	}

	if err := gd.doors[door].Trigger(force); err != nil {
		response.Success = false
		response.Data["message"] = "ERROR: " + err.Error()
		return nil
	}

	response.Success = true
	response.Data["message"] = "Garage door successfully triggered."
	response.Data["door"] = door

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
		log.Fatal(err)
	}

	configFile, err := ioutil.ReadFile(*configFileName)
	if err != nil {
		log.Fatalln(err)
	}

	config := new(Config)
	if err := json.Unmarshal(configFile, config); err != nil {
		log.Fatalln(err)
	}

	module := new(GarageDoors)
	module.configureModule(config)

	if err := serve(module); err != nil {
		log.Fatalln(err)
	}
}
