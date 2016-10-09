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
