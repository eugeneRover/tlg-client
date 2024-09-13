package client

import (
	"encoding/binary"
	"encoding/json"
	"io"
	"log/slog"
	"net"
	"slices"

	"golang.org/x/exp/rand"
)

const (
	RANDOM_STRING_CONTENT = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ1234567890"
)

// ////////////////////////////////////////////////////////////
type Client struct {
	network       string // net.Dial network
	address       string // net.Dial address
	client_id     int
	conn          net.Conn
	tlg           chan []byte
	listenersChan chan Listener
}

func NewClient(network, address string) (c *Client) {
	c = &Client{}

	c.network = network
	c.address = address
	c.tlg = make(chan []byte)
	c.listenersChan = make(chan Listener, 100)
	return
}

/*
Connects to tlg-cocket-proxy, receives tlg client_id, start receiving and dispatching messages
*/
func (c *Client) Start() (err error) {

	//connect
	c.conn, err = net.Dial(c.network, c.address)
	if err != nil {
		slog.Error("Cannot connect to tlg-socket-proxy", "error", err)
		return
	}

	//read client id
	id_buf := make([]byte, 4)
	_, err = io.ReadFull(c.conn, id_buf)
	if err != nil {
		slog.Error("Cannot read client id from tlg-socket-proxy", "error", err)
		return
	}
	c.client_id = int(binary.BigEndian.Uint32(id_buf))
	slog.Info("Received", "client id", c.client_id)

	// start receiver thread,
	go read_loop(c.conn, c.tlg)

	//start dispatcher
	go dispatcher(c.tlg, c.listenersChan)

	return
}

// start reading messages from tlg-socket-server
func read_loop(conn net.Conn, dest chan []byte) {
	for {
		// read 4 bytes - L=length of following message
		len_buf := make([]byte, 4)
		_, err := io.ReadFull(conn, len_buf)
		if err == nil {
			// read L bytes (see above) - message
			msg_len := binary.BigEndian.Uint32(len_buf)
			msg_buf := make([]byte, msg_len)
			_, err = io.ReadFull(conn, msg_buf)
			if err == nil {
				dest <- msg_buf
				continue
			}
		}

		if err == io.EOF {
			slog.Error("Server closed connection")
			close(dest)
			return
		}

		slog.Error("Cannot read incoming message, skipping", "error", err)
		continue
	}
}

func dispatcher(tlg <-chan []byte, listenersChan <-chan Listener) {
	var listeners []Listener //chain of responsibility

MainCycle:
	for {
		select {
		case l := <-listenersChan:
			listeners = append(listeners, l)

		case msg := <-tlg:
			m := make(Message)
			if json.Unmarshal(msg, &m) != nil {
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
		}
	}
}

func (c *Client) send(data []byte) (err error) {
	len_buf := make([]byte, 4)
	binary.BigEndian.PutUint32(len_buf, uint32(len(data)))

	if _, err = c.conn.Write(len_buf); err == nil {
		_, err = c.conn.Write(data)
	}
	return
}

func randStringBytes(n int) string {
	b := make([]byte, n)
	for i := range b {
		b[i] = RANDOM_STRING_CONTENT[rand.Intn(len(RANDOM_STRING_CONTENT))]
	}
	return string(b)
}

// asynchronous
func (c *Client) Send(msg *Message) (err error) {
	extra := randStringBytes(40)
	msg.SetExtra(&extra)

	if json, err := msg.Json(); err == nil {
		c.send(json)
	}
	return
}

// make async sync again (wait in the same thread)
func (c *Client) SendAndWait(msg *Message) (response *Message, err error) {
	extra := randStringBytes(40)
	msg.SetExtra(&extra)

	done := make(chan *Message, 1)
	c.AddClientExtraOnceListener(extra, func(m *Message) { done <- m })

	if json, err := msg.Json(); err == nil {
		c.send(json)
		response = <-done
	}
	return
}

func (c *Client) SendAndWait0(msg_type string) (response *Message, err error) {
	return c.SendAndWait(NewMessageType(msg_type))
}

func (c *Client) SendAndWait1(field1 string, value1 any) (response *Message, err error) {
	return c.SendAndWait(NewMessage1(field1, value1))
}

func (c *Client) SendAndWait2(field1 string, value1 any, field2 string, value2 any) (response *Message, err error) {
	return c.SendAndWait(NewMessage2(field1, value1, field2, value2))
}

func (c *Client) SendAndWait3(field1 string, value1 any, field2 string, value2 any, field3 string, value3 any) (response *Message, err error) {
	return c.SendAndWait(NewMessage3(field1, value1, field2, value2, field3, value3))
}

func (c *Client) SendAndWait4(field1 string, value1 any, field2 string, value2 any, field3 string, value3 any, field4 string, value4 any) (response *Message, err error) {
	return c.SendAndWait(NewMessage4(field1, value1, field2, value2, field3, value3, field4, value4))
}

func (c *Client) SendAndWait5(field1 string, value1 any, field2 string, value2 any, field3 string, value3 any, field4 string, value4 any, field5 string, value5 any) (response *Message, err error) {
	return c.SendAndWait(NewMessage5(field1, value1, field2, value2, field3, value3, field4, value4, field5, value5))
}

func (c *Client) ClientId() int {
	return int(c.client_id)
}

func (c *Client) AddListener(listener Listener) {
	c.listenersChan <- listener
}

func (c *Client) AddClientExtraOnceListener(extra string, handler func(*Message)) {
	c.AddListener(OnceListener{
		[]MessagePredicate{ClientIdPredicate(c.ClientId()), ExtraPredicate(extra)},
		handler,
	})
}

func (c *Client) AddNonRemovableListener(predicates []MessagePredicate, handler func(*Message)) {
	c.AddListener(NonRemovableListener{predicates, handler})
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
type OnceListener struct {
	Predicates []MessagePredicate //all predicates must return true for message to be handled
	Handler    func(*Message)
}

func (l OnceListener) Process(m *Message) ListenerResponse {
	for _, p := range l.Predicates {
		if !p(m) { //if at least 1 predicate returns false
			return NO_STOP_NO_REMOVE // do not handle
		}
	}
	go l.Handler(m)
	return STOP_REMOVE
}

/*
Non removable listener
*/
type NonRemovableListener struct {
	Predicates []MessagePredicate //all predicates must return true for message to be handled
	Handler    func(msg *Message)
}

func (l NonRemovableListener) Process(m *Message) ListenerResponse {
	for _, p := range l.Predicates {
		if !p(m) { //if at least 1 predicate returns false
			return NO_STOP_NO_REMOVE // do not handle
		}
	}
	go l.Handler(m)
	return STOP_NO_REMOVE
}

// //////////////////////////////////////////////////////////////////////////

type MessagePredicate func(m *Message) bool

func ClientIdPredicate(cId int) MessagePredicate {
	return func(m *Message) bool {
		return m.ClientId() == cId
	}
}

func OfTypesPredicate(s ...string) MessagePredicate {
	return func(m *Message) bool {
		return slices.Index(s, m.Type()) != -1
	}
}

func ExtraPredicate(extra string) MessagePredicate {
	return func(m *Message) bool {
		return m.Extra() == extra
	}
}
