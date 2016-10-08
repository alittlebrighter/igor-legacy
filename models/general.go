package models

import (
	"encoding/json"
)

type Request struct {
	Module string
	Method string
	Args   json.RawMessage
}

func NewRequest(module, method string, args interface{}) (*Request, error) {
	argData, err := json.Marshal(args)
	return &Request{Module: module, Method: method, Args: argData}, err
}

type Response struct {
	Module             string
	Success, Broadcast bool
	Data               map[string]interface{}
}

func NewResponse(module string) *Response {
	return &Response{Module: module, Success: false, Broadcast: false, Data: make(map[string]interface{})}
}

func NewErrorResponse(module, method, errorMsg string) *Response {
	return &Response{Module: module, Success: false, Broadcast: false,
		Data: map[string]interface{}{"method": method, "message": errorMsg}}
}
