package modules

import (
	"net"
	"net/rpc"

	"github.com/alittlebrighter/igor/models"
)

const (
	ModulePrefix = "igor.module."
)

type Module interface {
	Docs(models.Request, *models.Response) error
}

type BaseConfig struct {
	Name, SocketDir string
}

type BaseModule struct {
	Name, SocketDir string
}

func Serve(m Module, socketDir, mName string) error {
	listener, err := net.Listen("unix", socketDir+mName)
	if err != nil {
		return err
	}

	server := rpc.NewServer()
	server.RegisterName(mName, m)
	server.Accept(listener)
	return nil
}
