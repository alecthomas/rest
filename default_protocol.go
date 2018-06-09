package rest

import (
	"bytes"
	"encoding/json"
	"io/ioutil"
	"net/http"
)

// DefaultProtocol implements a default JSON protocol with a standard error format.
var DefaultProtocol Protocol = defaultProtocol{}

type defaultProtocol struct{}

func (d defaultProtocol) DecodeClientRequest(req *http.Request, v interface{}) error {
	return json.NewDecoder(req.Body).Decode(v)
}

func (d defaultProtocol) EncodeServerResponse(req *http.Request, w http.ResponseWriter, code int, err error, v interface{}) error {
	if err != nil {
		response, ok := err.(*ErrorResponse)
		if ok {
			code = response.Status
		} else {
			if code == 0 {
				code = http.StatusInternalServerError
			}
			response = &ErrorResponse{Status: code, Message: err.Error()}
		}
		return d.EncodeServerResponse(req, w, code, nil, response)
	}

	if code == 0 {
		if req.Method == "POST" {
			code = http.StatusCreated
		} else if v == nil {
			code = http.StatusNoContent
		} else {
			code = http.StatusOK
		}
	}
	w.Header().Add("Content-Type", "application/json")
	w.WriteHeader(code)
	return json.NewEncoder(w).Encode(v)
}

func (d defaultProtocol) EncodeClientRequest(req *http.Request, v interface{}) error {
	if v == nil {
		return nil
	}
	req.Header.Add("Content-Type", "application/json")
	req.Header.Add("Accept", "application/json")
	buf := &bytes.Buffer{}
	req.Body = ioutil.NopCloser(buf)
	return json.NewEncoder(buf).Encode(v)
}

func (d defaultProtocol) DecodeServerResponse(resp *http.Response, v interface{}) error {
	if resp.StatusCode < 400 {
		return json.NewDecoder(resp.Body).Decode(v)
	}
	errr := &ErrorResponse{}
	err := json.NewDecoder(resp.Body).Decode(errr)
	if err != nil {
		return err
	}
	return errr
}
