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
