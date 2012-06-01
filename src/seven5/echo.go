package seven5

import (
	"bytes"
	"fmt"
	"net/http"
	"seven5/util"
)

//EchoResult is the result type of a call on the Echo plugin.
//Must be public for json encoding.
type EchoResult struct {
    Error bool
	Body string
}

// Default echo plugin just prints an echo of the values it
// received in the request.
type DefaultEcho struct {
}

func (self *DefaultEcho) Exec(ignored1 string, ignored2 string,
	config *ApplicationConfig, request *http.Request,
	log util.SimpleLogger) interface{} {

	log.Info("this is a log message from the echo groupie")

	result := EchoResult{}
	var body bytes.Buffer

	body.WriteString("<H1>Echo To You</H1>")
	body.WriteString("<H3>Headers</H3>")
	for i, j := range request.Header {
		for _, k := range j {
			body.WriteString(fmt.Sprintf("<span>%s:%s</span><br/>", i, k))
		}
	}
	body.WriteString("<H3>Cookies</H3>")
	for _, cookie := range request.Cookies() {
		c := fmt.Sprintf("<span>%s,%s,%s,%s</span><br/>", cookie.Name,
			cookie.Expires.String(), cookie.Domain, cookie.Path)
		body.WriteString(c)
	}
	body.WriteString("<h3>Big Stuff</h3>")
	body.WriteString(fmt.Sprintf("<span>%s %s</span><br/>",
		request.Method, request.URL))
	values := request.URL.Query()
	if len(values) > 0 {
		body.WriteString("<h4>Query Params</h4>")
		for k, l := range values {
			for _, v := range l {
				body.WriteString(fmt.Sprintf("<span>%s:%s</span><br/>", k, v))
			}
		}
	}

	result.Body = body.String()
	result.Error = false
	return &result
}
