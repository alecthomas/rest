package rest

import (
	"fmt"
	"net/http"
)

// StatusCode is a type that can be returned by a handler to explicitly set a status code.
type StatusCode int

// ServerDecoder is used by the server to decode client requests.
type ServerDecoder interface {
	DecodeClientRequest(req *http.Request, v interface{}) error
}

// ServerEncoder is used by the server to encode responses.
type ServerEncoder interface {
	EncodeServerResponse(req *http.Request, w http.ResponseWriter, code int, err error, v interface{}) error
}

// ServerProtocol defines the protocol that the server conforms to.
type ServerProtocol interface {
	ServerDecoder
	ServerEncoder
}

// ClientEncoder is used by the client to encode requests.
type ClientEncoder interface {
	EncodeClientRequest(req *http.Request, v interface{}) error
}

// ClientEncoder is used by the client to decode responses.
type ClientDecoder interface {
	DecodeServerResponse(resp *http.Response, v interface{}) error
}

// ClientProtocol defines the protocol that a client conforms to.
type ClientProtocol interface {
	ClientEncoder
	ClientDecoder
}

// Protocol is both client and server protocol.
type Protocol interface {
	ServerProtocol
	ClientProtocol
}

// ErrorResponse is the response type returned in the body of HTTP errors (>= 400).
type ErrorResponse struct {
	Status  int    `json:"status"`
	Message string `json:"message"`
}

func (e *ErrorResponse) Error() string { return fmt.Sprintf("%d: %s", e.Status, e.Message) }

// Error creates a new HTTP error response.
func Error(code int, msg string) error { return &ErrorResponse{Status: code, Message: msg} }

// Errorf creates a new HTTP error response with Sprintf formatting.
func Errorf(code int, format string, args ...interface{}) error {
	return &ErrorResponse{Status: code, Message: fmt.Sprintf(format, args...)}
}
