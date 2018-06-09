package rest

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestREST(t *testing.T) {
	type testRequest struct {
		Message string
	}
	type testResponse struct {
		Message string
	}
	r := New()
	r.Get("/custom_error", func() error {
		return Errorf(http.StatusBadRequest, "invalid")
	})
	r.Get("/normal_error", func() error {
		return fmt.Errorf("error")
	})
	r.Get("/integer/:id", func(id int) (int, error) {
		return id + 33, nil
	})
	r.Get("/struct_response", func() (*testResponse, error) {
		return &testResponse{Message: "teapot"}, nil
	})
	r.Get("/override_status_code", func() (StatusCode, error) {
		return http.StatusTeapot, nil
	})
	r.Get("/override_status_code_with_body", func() (*testResponse, StatusCode, error) {
		return &testResponse{Message: "teapot"}, http.StatusTeapot, nil
	})
	r.Post("/request_body", func(req *testRequest) (*testResponse, error) {
		return &testResponse{Message: req.Message + " teapot"}, nil
	})

	server := httptest.NewServer(r)
	defer server.Close()

	t.Run("CustomError", func(t *testing.T) {
		actual := &ErrorResponse{}
		resp := getAndDecode(t, server, "/custom_error", actual)
		require.Equal(t, http.StatusBadRequest, resp.StatusCode)
		require.Equal(t, Error(http.StatusBadRequest, "invalid"), actual)
	})

	t.Run("NormalError", func(t *testing.T) {
		actual := &ErrorResponse{}
		resp := getAndDecode(t, server, "/normal_error", actual)
		require.Equal(t, http.StatusInternalServerError, resp.StatusCode)
		require.Equal(t, Error(http.StatusInternalServerError, "error"), actual)
	})

	t.Run("IntPathParam", func(t *testing.T) {
		out := 0
		getAndDecode(t, server, "/integer/10", &out)
		require.Equal(t, 43, out)
	})

	t.Run("StructResponse", func(t *testing.T) {
		actual := &testResponse{}
		resp := getAndDecode(t, server, "/struct_response", actual)
		require.Equal(t, http.StatusOK, resp.StatusCode)
		require.Equal(t, &testResponse{Message: "teapot"}, actual)
	})

	t.Run("OverrideStatusCode", func(t *testing.T) {
		resp := getAndDecode(t, server, "/override_status_code", nil)
		require.Equal(t, http.StatusTeapot, resp.StatusCode)
	})

	t.Run("OverrideStatusCodeWithBody", func(t *testing.T) {
		actual := &testResponse{}
		resp := getAndDecode(t, server, "/override_status_code_with_body", actual)
		require.Equal(t, http.StatusTeapot, resp.StatusCode)
		require.Equal(t, &testResponse{Message: "teapot"}, actual)
	})
	t.Run("PostRequestBody", func(t *testing.T) {
		req := &testRequest{Message: "hello"}
		actual := &testResponse{}
		resp := postAndDecode(t, server, "/request_body", req, actual)
		require.Equal(t, http.StatusCreated, resp.StatusCode)
		require.Equal(t, &testResponse{Message: "hello teapot"}, actual)
	})
}

func getAndDecode(t *testing.T, server *httptest.Server, path string, v interface{}) *http.Response {
	t.Helper()
	resp, err := server.Client().Get(server.URL + path)
	require.NoError(t, err)
	defer resp.Body.Close()
	if v != nil {
		err = json.NewDecoder(resp.Body).Decode(v)
		require.NoError(t, err)
	}
	return resp
}

func postAndDecode(t *testing.T, server *httptest.Server, path string, req interface{}, resp interface{}) *http.Response {
	t.Helper()
	w := &bytes.Buffer{}
	json.NewEncoder(w).Encode(req)
	rep, err := server.Client().Post(server.URL+path, "application/json", w)
	require.NoError(t, err)
	defer rep.Body.Close()
	if resp != nil {
		err = json.NewDecoder(rep.Body).Decode(resp)
		require.NoError(t, err)
	}
	return rep
}
