package common

const (
	ModulePrefix = "igor.module."
	ChanBuffer   = 5
)

type Request struct {
	Module string
	Method string
	Args   map[string]interface{}
}

type Response struct {
	Module             string
	Success, Broadcast bool
	Data               map[string]interface{}
}

func NewResponse(module string) *Response {
	return &Response{Module: module, Success: false, Broadcast: false, Data: make(map[string]interface{})}
}
