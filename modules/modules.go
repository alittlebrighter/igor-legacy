package modules

import (
	"net"
	"net/rpc"

	"github.com/alittlebrighter/igor/common"
)

type Module interface {
	Docs(common.Request, *common.Response) error
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
