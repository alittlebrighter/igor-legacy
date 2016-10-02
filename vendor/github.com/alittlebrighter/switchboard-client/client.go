package relayClient

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/alittlebrighter/switchboard/models"
	"github.com/alittlebrighter/switchboard/util"
	uuid "github.com/satori/go.uuid"
	"golang.org/x/net/websocket"

	"github.com/alittlebrighter/igor-relay-client/security"
)

const byteChunkSize = 256

type RelayClient struct {
	id           *uuid.UUID
	relayHost    string
	socketConn   *websocket.Conn
	marshaller   func(interface{}) ([]byte, error)
	unmarshaller func(data []byte, v interface{}) error
}

func (rc *RelayClient) Marshaller() func(interface{}) ([]byte, error) {
	return rc.marshaller
}

func (rc *RelayClient) Unmarshaller() func(data []byte, v interface{}) error {
	return rc.unmarshaller
}

func New(id *uuid.UUID, relayHost, keyfile string, marshaller func(interface{}) ([]byte, error), unmarshaller func(data []byte, v interface{}) error) *RelayClient {
	security.GenerateSharedKey(keyfile)
	return &RelayClient{id: id, relayHost: relayHost, marshaller: marshaller, unmarshaller: unmarshaller}
}

// OpenSocket attempts a connection to the public relay server via websockets
// should come up with a retry mechanism
func (rc *RelayClient) OpenSocket() error {
	// origin can be a bogus URL so we'll just use it to identify the connection on the server
	origin := "http://" + rc.id.String()
	url := "ws://" + rc.relayHost + "/socket"

	ws, err := websocket.Dial(url, "", origin)
	if err != nil {
		return err
	}
	rc.socketConn = ws
	return nil
}

// ReadMessages opens a websocket or polls on host arg identifying itself with controllerID arg and
// returns a channel that relays messages coming down from the server
func (rc *RelayClient) ReadMessages() (relayChan chan *models.Envelope, err error) {
	relayChan = make(chan *models.Envelope, 10)

	processMsg := func(data []byte) {
		env := new(models.Envelope)
		if err := util.Unmarshal(data, env); err != nil {
			log.Printf("Error parsing data: %s\n", err.Error())
		} else {
			relayChan <- env
		}
	}

	if rc.socketConn != nil {
		go util.ReadFromWebSocket(rc.socketConn, processMsg)
	} else {
		go func() {
			request, err := http.NewRequest(
				"GET",
				fmt.Sprintf("http://%s/messages?to="+rc.id.String(), rc.relayHost),
				nil)
			if err != nil {
				log.Println("Error building request: " + err.Error())
				return
			}
			response, err := http.DefaultClient.Do(request)
			if err != nil {
				log.Println("Error making request: " + err.Error())
				return
			}

			// download mailbox contents
			msgResponse, err := ioutil.ReadAll(io.LimitReader(response.Body, 1048576))
			var msgs []models.Envelope
			err = rc.unmarshaller(msgResponse, msgs)
			if err != nil {
				log.Println("Error parsing request: " + err.Error())
				return
			}

			for _, msg := range msgs {
				relayChan <- &msg
			}
			close(relayChan)
		}()
	}

	return
}

func (rc *RelayClient) SendMessage(env *models.Envelope) (msgResponse []byte, err error) {
	if rc.socketConn != nil { // && rc.socketConn.IsServerConn() {
		return rc.sendMessageWS(env)
	}

	return rc.sendMessageHTTP(env)
}

func (rc *RelayClient) sendMessageHTTP(env *models.Envelope) (msgResponse []byte, err error) {
	reqBody, err := rc.marshaller(env)

	request, err := http.NewRequest(
		"POST",
		fmt.Sprintf("http://%s/messages", rc.relayHost),
		bytes.NewBuffer(reqBody))
	if err != nil {
		return
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Content-Length", strconv.Itoa(len(reqBody)))
	request.ContentLength = int64(len(reqBody))

	response, err := http.DefaultClient.Do(request)
	if err != nil {
		return
	}

	msgResponse, err = ioutil.ReadAll(io.LimitReader(response.Body, 1048576))
	return
}

func (rc *RelayClient) sendMessageWS(env *models.Envelope) ([]byte, error) {
	reqBody, err := rc.marshaller(env)
	if err != nil {
		return nil, err
	}

	err = websocket.Message.Send(rc.socketConn, reqBody)
	return []byte("sent via websocket"), err
}

func (rc *RelayClient) NewEnvelope(to *uuid.UUID, expires *time.Time, contents interface{}) (env *models.Envelope, err error) {
	env = &models.Envelope{To: to, From: rc.id, Expires: expires}

	marshalled, err := rc.Marshaller()(contents)
	if err != nil {
		return
	}
	env.Contents = string(marshalled)

	if env.Contents, err = security.EncryptToString(marshalled); err != nil {
		return
	}

	env.Signature, err = security.SignToString(env.Contents)
	return
}
