package client

import (
	"encoding/json"
	"log/slog"
	"slices"
	"unsafe"

	"golang.org/x/exp/rand"
)

// #cgo pkg-config: tdjson
// #include <td/telegram/td_json_client.h>
// #include <stdlib.h>
import "C"

const (
	RECEIVE_TIMEOUT       = C.double(5) //sec
	RANDOM_STRING_CONTENT = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ1234567890"
)

// ////////////////////////////////////////////////////////////
type Client struct {
	client_id      C.int
	tlg            chan string
	listenersChan  chan Listener
	allOther       chan Message
	getTdlibParams func() map[string]interface{}
	getPhone       func() string
	getCode        func() string
	getPassword    func() string
}

func NewClient(
	getTdlibParams func() map[string]interface{},
	getPhone func() string,
	getCode func() string,
	getPassword func() string) (c *Client) {
	c = &Client{}

	c.client_id = C.td_create_client_id()
	c.Send(NewMessage("setLogVerbosityLevel", Message{"new_verbosity_level": 0}))
	c.Send(NewMessage("setLogStream", Message{"log_stream": Message{"@type": "logStreamEmpty"}}))

	c.tlg = make(chan string)
	c.listenersChan = make(chan Listener, 100)
	c.allOther = make(chan Message, 100)
	c.getTdlibParams = getTdlibParams
	c.getPhone = getPhone
	c.getCode = getCode
	c.getPassword = getPassword
	return
}

/*
Starts receiving and dispatching messages, and authorization flow
Returns channel that will receive something on authStateReady
*/
func (c *Client) Start() <-chan int {

	// c.listenersChan <- ClientTypeListener{"updateOption", c.client_id, func(m *Message) {
	// 	var v2 any
	// 	if v1, ok := (*m)["value"]; ok {
	// 		v2 = (v1.(map[string]interface{}))["value"]
	// 	}
	// 	slog.Info("UpdateOption", "name", (*m)["name"], "value", v2)
	// }}

	authorized := make(chan int)
	c.AddListener(*c.NewTypeListener([]string{"updateAuthorizationState"}, c.updateAuthState(authorized)))

	// start receiver thread,
	go func(out chan<- string) {
		for {
			if result := C.td_receive(RECEIVE_TIMEOUT); result != nil {
				out <- C.GoString(result)
			}
		}
	}(c.tlg)

	//start dispatcher
	go dispatcher(c.tlg, c.listenersChan, c.allOther)

	//start unhandled messages reader thread
	go func(inp <-chan Message) {
		for range inp {
			// slog.Info("Not handled", "message", msg)
		}
	}(c.allOther)

	return authorized
}

func dispatcher(tlg <-chan string, listenersChan <-chan Listener, allOther chan<- Message) {
	var listeners []Listener //chain of responsibility

MainCycle:
	for {
		select {
		case l := <-listenersChan:
			listeners = append(listeners, l)

		case msg := <-tlg:
			m := make(Message)
			if json.Unmarshal([]byte(msg), &m) != nil {
				slog.Error("Unable to unmarchal incoming message", "json", msg)
				continue
			}

			for i, l := range listeners {
				switch l.Process(&m) {
				case STOP_REMOVE:
					listeners = slices.Delete(listeners, i, i+1) // remove this listener
					fallthrough
				case STOP_NO_REMOVE:
					continue MainCycle // receive next message
				}
			}

			// no listener desired to stop processing, sending to default chan
			allOther <- m
		}
	}
}

func send(client_id C.int, jq string) {

	query := C.CString(jq)
	defer C.free(unsafe.Pointer(query))

	C.td_send(client_id, query)
}

func execute(jq string) string {
	query := C.CString(jq)
	defer C.free(unsafe.Pointer(query))

	cs := C.td_execute(query)
	return C.GoString(cs)
}

func randStringBytes(n int) string {
	b := make([]byte, n)
	for i := range b {
		b[i] = RANDOM_STRING_CONTENT[rand.Intn(len(RANDOM_STRING_CONTENT))]
	}
	return string(b)
}

func (c *Client) updateAuthState(authReady chan<- int) func(m *Message, msg_type string) {
	return func(m *Message, _ string) {
		switch m.Fstring("authorization_state.@type") {

		case "authorizationStateWaitTdlibParameters":
			resp, _ := c.SendAndWait(NewMessage("setTdlibParameters", c.getTdlibParams()))
			if !resp.IsOk() {
				slog.Error("Unable to setTdlibParameters", "answer", resp)
			}
		case "authorizationStateWaitPhoneNumber":
			resp, _ := c.SendAndWait(NewMessage("setAuthenticationPhoneNumber", map[string]interface{}{
				"phone_number": c.getPhone(),
			}))
			if !resp.IsOk() {
				slog.Error("Unable to setAuthenticationPhoneNumber", "answer", resp)
			}
		case "authorizationStateWaitCode":
			resp, _ := c.SendAndWait(NewMessage("checkAuthenticationCode", map[string]interface{}{
				"code": c.getCode(),
			}))
			if !resp.IsOk() {
				slog.Error("Unable to checkAuthenticationCode", "answer", resp)
			}

		case "authorizationStateWaitPassword":
			resp, _ := c.SendAndWait(NewMessage("checkAuthenticationPassword", map[string]interface{}{
				"password": c.getPassword(),
			}))
			if !resp.IsOk() {
				slog.Error("Unable to checkAuthenticationPassword", "answer", resp)
			}
		case "authorizationStateReady":
			authReady <- 1
		}

	}
}

// asynchronous
func (c *Client) Execute(msg *Message) (result *Message, err error) {
	js, err := msg.Json()
	if err != nil {
		return
	}
	s := []byte(execute(string(js)))
	m := make(Message)
	err = json.Unmarshal(s, &m)

	result = &m
	return
}

// asynchronous
func (c *Client) Send(msg *Message) (err error) {
	extra := randStringBytes(40)
	msg.SetExtra(&extra)

	if json, err := msg.Json(); err == nil {
		send(c.client_id, string(json))
	}
	return
}

// make async sync again (wait in the same thread)
func (c *Client) SendAndWait(msg *Message) (response *Message, err error) {
	extra := randStringBytes(40)
	msg.SetExtra(&extra)

	done := make(chan *Message, 1)
	c.listenersChan <- *c.NewExtraOnceListener(extra, func(m *Message) { done <- m })

	if json, err := msg.Json(); err == nil {
		send(c.client_id, string(json))
		response = <-done
	}
	return
}

func (c *Client) ClientId() int {
	return int(c.client_id)
}

func (c *Client) AddListener(listener Listener) {
	c.listenersChan <- listener
}

func (c *Client) NewExtraOnceListener(extra string, handler func(*Message)) *ClientExtraOnceListener {
	return &ClientExtraOnceListener{extra, c.ClientId(), handler}
}

func (c *Client) NewTypeListener(_types []string, handler func(*Message, string)) *ClientTypeListener {
	return &ClientTypeListener{_types, c.ClientId(), handler}
}

// //////////////////////////////////////////////////////////////////////////
type ListenerResponse uint8

const (
	NO_STOP_NO_REMOVE ListenerResponse = 0 //do not stop processing, and dont remove this listener
	STOP_NO_REMOVE    ListenerResponse = 1 // stop processing, and dont remove this listener
	STOP_REMOVE       ListenerResponse = 2 // stop processing, and remove this listener
)

type Listener interface {
	Process(m *Message) ListenerResponse
}

/*
Listener that removes itself after a single hit
*/
type ClientExtraOnceListener struct {
	Extra    string
	ClientId int
	Handler  func(*Message)
}

func (l ClientExtraOnceListener) Process(m *Message) ListenerResponse {
	if m.Extra() != l.Extra || m.ClientId() != l.ClientId {
		return NO_STOP_NO_REMOVE
	}

	go l.Handler(m)
	return STOP_REMOVE
}

/*
Non removable listener
*/
type ClientTypeListener struct {
	Types    []string
	ClientId int
	Handler  func(msg *Message, msg_type string)
}

func (l ClientTypeListener) Process(m *Message) ListenerResponse {
	mtype := m.Type() // it is assumed that @type field exists here
	if m.ClientId() != l.ClientId || slices.Index(l.Types, mtype) == -1 {
		return NO_STOP_NO_REMOVE
	}

	go l.Handler(m, mtype)
	return STOP_NO_REMOVE
}

// //////////////////////////////////////////////////////////////////////////
