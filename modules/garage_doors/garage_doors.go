package main

import (
	"time"

	"github.com/kidoman/embd"
	_ "github.com/kidoman/embd/host/rpi"
)

const (
	triggerTimeUnit = time.Millisecond
)

type GarageDoorController struct {
	pin              embd.DigitalPin
	triggered        bool
	triggerTime      time.Duration
	forceTriggerTime time.Duration
	cancel           chan bool
}

func NewGarageDoorController(pin int, triggerTime, forceTime time.Duration) (controller *GarageDoorController, err error) {
	controller = &GarageDoorController{
		triggered:        false,
		triggerTime:      triggerTime * triggerTimeUnit,
		forceTriggerTime: forceTime * triggerTimeUnit,
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
			timeout.Stop()
		case <-timeout.C:
		}
	}()

	return
}
