package util

import (
	"encoding/json"
	"encoding/xml"
	"io"
	"io/ioutil"
	"net/http"
	"strings"

	"golang.org/x/net/websocket"
)

type Unmarshaller func([]byte, interface{}) error

func Unmarshal(data []byte, dest interface{}) error {
	return getUnmarshallerByMimeType(http.DetectContentType(data))(data, dest)
}

// UnmarshalRequest determines the content type of the body of a request by first checking
// the content-type header and then falling back to scanning the body to unmarshal into dest
func UnmarshalRequest(r *http.Request, dest interface{}) error {
	data, err := ioutil.ReadAll(io.LimitReader(r.Body, 1048576))
	if err != nil {
		return err
	}

	var mimeType string
	switch {
	case r.Header.Get("Content-Type") != "":
		mimeType = r.Header.Get("Content-Type")
	default:
		mimeType = http.DetectContentType(data)
	}

	return getUnmarshallerByMimeType(mimeType)(data, dest)
}

func getUnmarshallerByMimeType(mimeType string) (unmarshaller Unmarshaller) {
	switch {
	case strings.HasSuffix(mimeType, "json"):
		unmarshaller = json.Unmarshal
	case strings.HasSuffix(mimeType, "xml"):
		unmarshaller = xml.Unmarshal
	default:
		unmarshaller = json.Unmarshal
	}
	return
}

type Marshaller func(v interface{}) ([]byte, error)

// MarshalResponse acts as a universal Marshaller trying to give the client of an http request what it wants
// by checking the accept and content-type headers before defaulting to JSON
func MarshalResponse(r *http.Request, v interface{}) ([]byte, error) {
	var mimeType string
	switch {
	case r.Header.Get("Accept") != "":
		mimeType = r.Header.Get("Accept")
	case r.Header.Get("Content-Type") != "":
		mimeType = r.Header.Get("Content-Type")
	default:
		mimeType = "application/json"
	}

	return getMarshallerByMimeType(mimeType)(v)
}

func MarshalToMimeType(v interface{}, mimeType string) ([]byte, error) {
	return getMarshallerByMimeType(mimeType)(v)
}

func getMarshallerByMimeType(mimeType string) (marshaller Marshaller) {
	switch {
	case strings.HasSuffix(mimeType, "json"):
		marshaller = json.Marshal
	case strings.HasSuffix(mimeType, "xml"):
		marshaller = xml.Marshal
	default:
		marshaller = json.Marshal
	}
	return
}

func ReadFromWebSocket(ws *websocket.Conn, processMsg func([]byte)) error {
	var err error
	chunkSize := 256

	for {
		msg := []byte{}
		n := chunkSize
		for n == chunkSize {
			chunk := make([]byte, chunkSize)
			if n, err = ws.Read(chunk); err != nil {
				break
			}
			msg = append(msg, chunk[:n]...)
		}
		if err != nil {
			break
		}

		processMsg(msg)
	}
	return err
}
