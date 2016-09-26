package common

import (
	"github.com/nats-io/nats"
)

const (
	ModulePrefix = "igor.module."
	ChanBuffer   = 5
)

type conns struct {
	Conn    *nats.Conn
	EncConn *nats.EncodedConn
}

var (
	privateRelay = conns{}
)

func PrivateRelayConn() *nats.Conn {
	return privateRelay.Conn
}

func SetPrivateRelayConn(conn *nats.Conn) {
	privateRelay.Conn = conn
}

func PrivateRelayEncConn() *nats.EncodedConn {
	return privateRelay.EncConn
}

func SetPrivateRelayEncConn(conn *nats.EncodedConn) {
	privateRelay.EncConn = conn
}

type Request struct {
	Module string
	Method string
	Args   map[string]interface{}
}

type Response struct {
	Module  string
	Success bool
	Data    map[string]interface{}
}

func NewResponse(module string) *Response {
	return &Response{Module: module, Success: false, Data: make(map[string]interface{})}
}
