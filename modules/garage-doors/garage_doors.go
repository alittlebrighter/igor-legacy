package garageDoors

import (
	"log"
	"strings"
	"time"

	"github.com/kidoman/embd"
	_ "github.com/kidoman/embd/host/rpi"

	modules "github.com/alittlebrighter/igor/common"
)

const (
	Name            = "garage-doors"
	triggerTimeUnit = time.Millisecond
)

var doors map[string]*GarageDoorController

type Config struct {
	Pins                          map[string]interface{}
	TriggerTime, ForceTriggerTime int
}

type GarageDoorController struct {
	pin              embd.DigitalPin
	triggered        bool
	triggerTime      time.Duration
	forceTriggerTime time.Duration
	cancel           chan bool
}

func NewGarageDoorController(pin, triggerTime, forceTime int) (controller *GarageDoorController, err error) {
	controller = &GarageDoorController{
		triggered:        false,
		triggerTime:      time.Duration(triggerTime) * triggerTimeUnit,
		forceTriggerTime: time.Duration(forceTime) * triggerTimeUnit,
		cancel:           make(chan bool)}
	controller.pin, err = embd.NewDigitalPin(pin)
	if err != nil {
		return
	}
	controller.pin.SetDirection(embd.Out)
	controller.pin.Write(embd.High)
	return
}

func (controller *GarageDoorController) setTriggered(triggered bool) {
	controller.triggered = triggered
}

func (controller *GarageDoorController) Trigger(force bool) (err error) {
	if controller.triggered {
		controller.cancel <- true
		return
	}

	go func() {
		controller.setTriggered(true)
		defer controller.setTriggered(false)

		err = controller.pin.Write(embd.Low)
		if err != nil {
			return
		}
		defer controller.pin.Write(embd.High)

		triggerTime := controller.triggerTime
		if force {
			triggerTime = controller.forceTriggerTime
		}

		timeout := time.NewTimer(triggerTime)

		select {
		case <-controller.cancel:
			log.Println("Trigger canceled")
			timeout.Stop()
		case <-timeout.C:
			log.Println("Trigger complete")
		}
	}()

	return
}

func parseConfig(configMap map[string]interface{}) *Config {
	config := new(Config)
	pins := configMap["pins"]
	config.Pins = pins.(map[string]interface{})
	tTime := configMap["triggerTime"]
	config.TriggerTime = int(tTime.(float64))
	ftTime := configMap["forceTriggerTime"]
	config.ForceTriggerTime = int(ftTime.(float64))
	return config
}

func Run(config map[string]interface{}, commands chan *modules.Request) (chan *modules.Response, error) {
	conf := parseConfig(config)

	doors = make(map[string]*GarageDoorController)
	for label, pin := range conf.Pins {
		controller, err := NewGarageDoorController(int(pin.(float64)), conf.TriggerTime, conf.ForceTriggerTime)
		if err != nil {
			return nil, err
		}
		doors[label] = controller
	}

	outbox := make(chan *modules.Response, modules.ChanBuffer)

	go runService(commands, outbox)

	return outbox, nil
}

func runService(commands chan *modules.Request, outbox chan *modules.Response) {
	defer embd.CloseGPIO()
	defer close(outbox)

	for req := range commands {
		response := modules.NewResponse(Name)

		if strings.ToLower(req.Method) != "trigger" {
			response.Success = false
			response.Data["message"] = "Method not found."
			outbox <- response
			continue
		}

		reqDoor, ok := req.Args["door"]
		if !ok {
			response.Success = false
			response.Data["message"] = "You must specify a door to trigger."
			outbox <- response
			continue
		}
		door := reqDoor.(string)

		var force bool
		if reqForce, ok := req.Args["force"]; ok {
			force = reqForce.(bool)
		}

		if err := doors[door].Trigger(force); err != nil {
			response.Success = false
			response.Data["message"] = "ERROR: " + err.Error()
			outbox <- response
			continue
		}
		response.Success = true
		outbox <- response
	}
}
