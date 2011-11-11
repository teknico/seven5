package seven5

import (
	"bytes"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"github.com/alecthomas/gozmq"
	"io"
	"strconv"
	"strings"
)

//Mongrel is a low-level abstraction for connecting, via 0MQ, to a mongrel2 server.
//Because this abstraction uses 0MQ and Mongrel2's (small) protocol on top of it,
//it is possible to do N:M communication by creating several of these structs with
//different identities.  See http://mongrel2.org/static/mongrel2-manual.html#x1-680005.1.7
type Mongrel2 struct {
	Context                     gozmq.Context
	InSocket, OutSocket         gozmq.Socket
	PullSpec, PubSpec, Identity string
}

//Request structs are the "raw" information sent to the handler by the Mongrel2 server.
//The primary fields of the mongrel2 protocol are broken out in this struct and the
//headers (supplied by the client, passed through by Mongrel2) are included as a map.
//The RawRequest slice is the byte slice that holds all the data.  The Body byte slice
//points to the same underlying storage.  The other fields, for convenienced have been
//parsed and _copied_ out of the RawRequest byte slice.
type Request struct {
	RawRequest []byte
	Body       []byte
	ServerId   string
	ClientId   int
	BodySize   int
	Path       string
	Header     map[string]string
}

//Response structss are sent back to Mongrel2 servers. The Mongrel2 server you wish
//to target should be specified with the UUID and the client of that server you wish
//to target should be in the ClientId field.  Note that this is a slice since you
//can target up to 128 clients with a single Response struct.  The other fields are
//passed through at the HTTP level to the client or clients.  There is no need to 
//set the Content-Length header as this is added automatically.  The easiest way
//to correctly target a Response is by looking at the values supplied in a Request
//struct.
type Response struct {
	UUID       string
	ClientId   []int
	Body       string
	StatusCode int
	StatusMsg  string
	Header     map[string]string
}

//initZMQ creates the necessary ZMQ machinery and sets the fields of the
//Mongrel2 struct.
func (self *Mongrel2) initZMQ() error {

	c, err := gozmq.NewContext()
	if err != nil {
		return err
	}
	self.Context = c

	s, err := self.Context.NewSocket(gozmq.PULL)
	if err != nil {
		return err
	}
	self.InSocket = s

	err = self.InSocket.Connect(self.PullSpec)
	if err != nil {
		return err
	}

	s, err = self.Context.NewSocket(gozmq.PUB)
	if err != nil {
		return err
	}
	self.OutSocket = s

	err = self.OutSocket.SetSockOptString(gozmq.IDENTITY, self.Identity)
	if err != nil {
		return err
	}

	err = self.OutSocket.Connect(self.PubSpec)
	if err != nil {
		return err
	}

	return nil
}

//NewMongrel2 creates a Mongrel2 struct that can handle requests from a Mongrel2
//server.  The pullSpec and pubSpec parameters must be the same as those supplied
//in the Mongrel2 configuration.  The ID string is a unique id associated with this
//handler;  normally this can safely be the result of calling Type4UUID().
func NewMongrel2(pullSpec string, pubSpec string, id string) *Mongrel2 {

	result := new(Mongrel2)
	result.PullSpec = pullSpec
	result.PubSpec = pubSpec
	result.Identity = id
	result.initZMQ()
	return result
}

//ReadMessage creates a new Request struct based on the values sent from a Mongrel2
//instance. This call blocks until it receives a Request.  Note that you can have
//several different Mongrel2 structs all waiting on messages and they will be 
//delivered in a round-robin fashion.  This call tries to be efficient and look
//at each byte only when necessary.  The body of the request is not examined by
//this method.
func (self *Mongrel2) ReadMessage() (*Request, error) {
	req, err := self.InSocket.Recv(0)
	if err != nil {
		return nil, err
	}

	endOfServerId := readSome(' ', req, 0)
	serverId := string(req[0:endOfServerId])

	endOfClientId := readSome(' ', req, endOfServerId+1)
	clientId, err := strconv.Atoi(string(req[endOfServerId+1 : endOfClientId]))
	if err != nil {
		return nil, err
	}

	endOfPath := readSome(' ', req, endOfClientId+1)
	path := string(req[endOfClientId+1 : endOfPath])

	endOfJsonSize := readSome(':', req, endOfPath+1)
	jsonSize, err := strconv.Atoi(string(req[endOfPath+1 : endOfJsonSize]))
	if err != nil {
		return nil, err
	}

	jsonMap := make(map[string]string)
	jsonStart := endOfJsonSize + 1

	if jsonSize > 0 {
		err = json.Unmarshal(req[jsonStart:jsonStart+jsonSize], &jsonMap)
		if err != nil {
			return nil, err
		}
	}

	bodySizeStart := (jsonSize + 1) + jsonStart
	bodySizeEnd := readSome(':', req, bodySizeStart)
	bodySize, err := strconv.Atoi(string(req[bodySizeStart:bodySizeEnd]))

	if err != nil {
		return nil, err
	}

	result := new(Request)
	result.RawRequest = req
	result.Body = req[bodySizeStart:bodySizeEnd]
	result.Path = path
	result.BodySize = bodySize
	result.ServerId = serverId
	result.ClientId = clientId
	result.Header = jsonMap

	return result, nil
}

func readSome(terminationChar byte, req []byte, start int) int {
	result := start
	for {
		if req[result] == terminationChar {
			break
		}
		result++
	}
	return result
}

//WriteMessage takes a Response structs and enques it for transmission.  This call 
//does _not_ block.  The Response struct must be targeted for a specific server
//(ServerId) and one or more clients (ClientID).  The Response object may be received
//by many Mongrel2 instances, but only the addressed instance will transmit the
//response on to the client or clients.
func (self *Mongrel2) WriteMessage(response *Response) error {
	c := make([]string, len(response.ClientId), len(response.ClientId))
	for i, x := range response.ClientId {
		c[i] = strconv.Itoa(x)
	}
	clientList := strings.Join(c, " ")

	//create the properly mangled body in HTTP format
	buffer := new(bytes.Buffer)
	if response.StatusMsg == "" {
		buffer.WriteString(fmt.Sprintf("HTTP/1.1 %d %s\r\n", 200, "OK"))
	} else {
		buffer.WriteString(fmt.Sprintf("HTTP/1.1 %d %s\r\n", response.StatusCode, response.StatusMsg))
	}

	buffer.WriteString(fmt.Sprintf("Content-Length: %d\r\n", len(response.Body)))

	for k, v := range response.Header {
		buffer.WriteString(fmt.Sprintf("%s: %s\r\n", k, v))
	}

	//critical, separating extra newline
	buffer.WriteString("\r\n")
	//then the body
	buffer.WriteString(response.Body)

	//now we have the true size the body and can put it all together
	msg := fmt.Sprintf("%s %d:%s, %s", response.UUID, len(clientList), clientList, buffer.String())

	buffer = new(bytes.Buffer)
	buffer.WriteString(msg)

	err := self.OutSocket.Send(buffer.Bytes(), 0)
	return err
}
//Type4UUID generates a RFC 4122 compliant UUID.  This code was originally posted
//by Ross Cox to the go-nuts mailing list.
//http://groups.google.com/group/golang-nuts/msg/5ebbdd72e2d40c09
func Type4UUID() (string, error) {
	b := make([]byte, 16)
	_, err := io.ReadFull(rand.Reader, b)
	if err != nil {
		return "", err
	}
	b[6] = (b[6] & 0x0F) | 0x40
	b[8] = (b[8] &^ 0x40) | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[:4], b[4:6], b[6:8], b[8:10], b[10:]), nil
}

//Shutdown cleans up the resources associated with this mongrel2 connection.
//Normally this function should be part of a defer call that is immediately after
//allocating the resources, like this:
//	mongrel:=NewMongrel(...)
//  defer mongrel.Shutdown()
func (self *Mongrel2) Shutdown() error {
	if err := self.InSocket.Close(); err != nil {
		return err
	}
	if err := self.OutSocket.Close(); err != nil {
		return err
	}
	self.Context.Close()
	return nil
}
