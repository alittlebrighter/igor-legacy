package modules

import (
	"github.com/alittlebrighter/igor/common"
	"github.com/alittlebrighter/igor/modules/garage-doors"
	"github.com/alittlebrighter/igor/modules/gateway"
)

type RunModule func(map[string]interface{}, chan *common.Request) (chan *common.Response, error)

var (
	Loaded = map[string]RunModule{
		gateway.Name:     gateway.Run,
		garageDoors.Name: garageDoors.Run,
	}
)
