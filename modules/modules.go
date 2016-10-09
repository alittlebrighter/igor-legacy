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
