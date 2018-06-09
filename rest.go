package rest

import (
	"context"
	"net/http"
	"reflect"
	"strconv"
	"strings"

	"github.com/bmizerany/pat"
)

var (
	errorType   = reflect.TypeOf((*error)(nil)).Elem()
	contextType = reflect.TypeOf((*context.Context)(nil)).Elem()
	requestType = reflect.TypeOf(&http.Request{})
	intType     = reflect.TypeOf(int(0))
	int8Type    = reflect.TypeOf(int8(0))
	int16Type   = reflect.TypeOf(int16(0))
	int32Type   = reflect.TypeOf(int32(0))
	int64Type   = reflect.TypeOf(int64(0))
	uintType    = reflect.TypeOf(uint(0))
	uint8Type   = reflect.TypeOf(uint8(0))
	uint16Type  = reflect.TypeOf(uint16(0))
	uint32Type  = reflect.TypeOf(uint32(0))
	uint64Type  = reflect.TypeOf(uint64(0))
	float32Type = reflect.TypeOf(float32(0))
	float64Type = reflect.TypeOf(float64(0))
)

type route struct {
	method  string
	path    string
	handler interface{}
}

type paramBuilder func(r *http.Request) (reflect.Value, error)

// A Router maps URLs to functions using the following rules.
//
// The first parameter may be neither or one of type context.Context or *http.Request.
// All path variables are then mapped to subsequent function parameters.
//
// Finally, if the routes method is a POST, PUT or PATCH, the request body will be decoded
// into the last parameter via ServerProtocol.DecodeClientRequest().
//
// The return type of the function may be either (error), (<body>, error), (StatusCode, error)
// or (<body>, StatusCode, error).
// If a <body> is returned, it is encoded using ServerProtocol.EncodeServerResponse().
type Router struct {
	router   *pat.PatternServeMux
	protocol Protocol
	routes   []route
}

// An Option to configure the Router.
type Option func(r *Router)

// WithProtocol is an option to configure the router's Protocol.
func WithProtocol(protocol Protocol) Option {
	return func(r *Router) {
		r.protocol = protocol
	}
}

// New creates a new Router. See Router for details.
//
// DefaultProtocol will be used if protocol is nil.
func New(options ...Option) *Router {
	r := &Router{protocol: DefaultProtocol, router: pat.New()}
	for _, option := range options {
		option(r)
	}
	return r
}

func (r *Router) returnError(req *http.Request, w http.ResponseWriter, code int, err error) {
	// TODO: Log this somehow.
	r.protocol.EncodeServerResponse(req, w, code, err, nil) // nolint
}

// Add manually adds a route.
func (r *Router) Add(method, path string, f interface{}) *Router {
	handler := r.buildHandler(method, path, f)
	r.router.Add(method, path, handler)
	return r
}

func (r *Router) Del(path string, f interface{}) *Router {
	return r.Add("DEL", path, f)
}

func (r *Router) Get(path string, f interface{}) *Router {
	return r.Add("GET", path, f)
}

func (r *Router) Head(path string, f interface{}) *Router {
	return r.Add("HEAD", path, f)
}

func (r *Router) Options(path string, f interface{}) *Router {
	return r.Add("OPTIONS", path, f)
}

func (r *Router) Patch(path string, f interface{}) *Router {
	return r.Add("PATCH", path, f)
}

func (r *Router) Post(path string, f interface{}) *Router {
	return r.Add("POST", path, f)
}

func (r *Router) Put(path string, f interface{}) *Router {
	return r.Add("PUT", path, f)
}

func (r *Router) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	r.router.ServeHTTP(w, req)
}

func (r *Router) buildHandler(method string, path string, f interface{}) http.HandlerFunc {
	r.routes = append(r.routes, route{method: method, path: path, handler: f})
	fv := reflect.ValueOf(f)
	ft := fv.Type()
	if ft.NumOut() == 0 {
		panic("expected return signature of (..., error) but got " + ft.String())
	}
	if ft.Out(ft.NumOut()-1) != errorType {
		panic("expected return signature of (..., error) but got (..., " + ft.Out(ft.NumOut()-1).String() + ") but got " + ft.String())
	}
	builders := []paramBuilder{}
	paramIndex := 0
	params := []string{}
	for _, part := range strings.Split(path, "/") {
		if strings.HasPrefix(part, ":") {
			params = append(params, part[1:])
		}
	}
	haveBody := false
	for i := 0; i < ft.NumIn(); i++ {
		pt := ft.In(i)
		var builder paramBuilder
		if pt == contextType {
			builder = func(r *http.Request) (reflect.Value, error) {
				return reflect.ValueOf(r.Context()), nil
			}
		} else if pt == requestType {
			builder = func(r *http.Request) (reflect.Value, error) {
				return reflect.ValueOf(r), nil
			}
		} else {
			if paramIndex < len(params) {
				builder = r.pathParamBuilder(pt, params[paramIndex], paramIndex)
				paramIndex++
			} else {
				if haveBody {
					panic("have already mapped all path parameters and request body, but have arguments remaining in " + ft.String())
				}
				if pt.Kind() == reflect.Ptr {
					pt = pt.Elem()
				}
				builder = func(req *http.Request) (reflect.Value, error) {
					v := reflect.New(pt)
					return v, r.protocol.DecodeClientRequest(req, v.Interface())
				}
				haveBody = true
			}
		}
		if builder == nil {
			panic("could not determine decoder for type " + pt.String())
		}
		builders = append(builders, builder)
	}
	return func(w http.ResponseWriter, req *http.Request) {
		// Build parameters.
		var err error
		params := make([]reflect.Value, len(builders))
		for i, builder := range builders {
			params[i], err = builder(req)
			if err != nil {
				r.returnError(req, w, http.StatusUnprocessableEntity, err)
				return
			}
		}
		ret := fv.Call(params)
		switch len(ret) {
		case 1: // (error)
			err := ret[0].Interface()
			if err != nil {
				r.protocol.EncodeServerResponse(req, w, 0, err.(error), nil)
			} else {
				r.protocol.EncodeServerResponse(req, w, 0, nil, nil)
			}

		case 2:
			err := ret[1].Interface()
			if err != nil {
				r.protocol.EncodeServerResponse(req, w, 0, err.(error), nil)
			} else if ret[0].Type() == reflect.TypeOf(StatusCode(0)) {
				r.protocol.EncodeServerResponse(req, w, int(ret[0].Interface().(StatusCode)), nil, nil)
			} else {
				body := ret[0].Interface()
				r.protocol.EncodeServerResponse(req, w, 0, nil, body)
			}
		case 3:
			err := ret[2].Interface()
			if err != nil {
				r.protocol.EncodeServerResponse(req, w, 0, err.(error), nil)
			} else {
				code := int(ret[1].Int())
				body := ret[0].Interface()
				r.protocol.EncodeServerResponse(req, w, code, nil, body)
			}
		}
	}
}

func (r *Router) pathParamBuilder(pt reflect.Type, paramName string, paramIndex int) paramBuilder {
	switch pt.Kind() {
	case reflect.String:
		return func(r *http.Request) (reflect.Value, error) {
			return reflect.ValueOf(r.URL.Query().Get(":" + paramName)), nil
		}
	case reflect.Float32:
		return func(r *http.Request) (reflect.Value, error) {
			var v reflect.Value
			n, err := strconv.ParseFloat(r.URL.Query().Get(":"+paramName), 32)
			if err == nil {
				v = reflect.New(float32Type).Elem()
				v.SetFloat(n)
			}
			return v, err
		}
	case reflect.Float64:
		return func(r *http.Request) (reflect.Value, error) {
			var v reflect.Value
			n, err := strconv.ParseFloat(r.URL.Query().Get(":"+paramName), 64)
			if err == nil {
				v = reflect.New(float64Type).Elem()
				v.SetFloat(n)
			}
			return v, err
		}
	case reflect.Int:
		return func(r *http.Request) (reflect.Value, error) {
			var v reflect.Value
			n, err := strconv.ParseInt(r.URL.Query().Get(":"+paramName), 10, 64)
			if err == nil {
				v = reflect.New(intType).Elem()
				v.SetInt(n)
			}
			return v, err
		}
	case reflect.Int8:
		return func(r *http.Request) (reflect.Value, error) {
			var v reflect.Value
			n, err := strconv.ParseInt(r.URL.Query().Get(":"+paramName), 10, 8)
			if err == nil {
				v = reflect.New(int8Type).Elem()
				v.SetInt(n)
			}
			return v, err
		}
	case reflect.Int16:
		return func(r *http.Request) (reflect.Value, error) {
			var v reflect.Value
			n, err := strconv.ParseInt(r.URL.Query().Get(":"+paramName), 10, 16)
			if err == nil {
				v = reflect.New(int16Type).Elem()
				v.SetInt(n)
			}
			return v, err
		}
	case reflect.Int32:
		return func(r *http.Request) (reflect.Value, error) {
			var v reflect.Value
			n, err := strconv.ParseInt(r.URL.Query().Get(":"+paramName), 10, 32)
			if err == nil {
				v = reflect.New(int32Type).Elem()
				v.SetInt(n)
			}
			return v, err
		}
	case reflect.Int64:
		return func(r *http.Request) (reflect.Value, error) {
			var v reflect.Value
			n, err := strconv.ParseInt(r.URL.Query().Get(":"+paramName), 10, 64)
			if err == nil {
				v = reflect.New(int64Type).Elem()
				v.SetInt(n)
			}
			return v, err
		}
	case reflect.Uint:
		return func(r *http.Request) (reflect.Value, error) {
			var v reflect.Value
			n, err := strconv.ParseUint(r.URL.Query().Get(":"+paramName), 10, 64)
			if err == nil {
				v := reflect.New(uintType).Elem()
				v.SetUint(n)
			}
			return v, err
		}
	case reflect.Uint8:
		return func(r *http.Request) (reflect.Value, error) {
			var v reflect.Value
			n, err := strconv.ParseUint(r.URL.Query().Get(":"+paramName), 10, 8)
			if err == nil {
				v := reflect.New(uint8Type).Elem()
				v.SetUint(n)
			}
			return v, err
		}
	case reflect.Uint16:
		return func(r *http.Request) (reflect.Value, error) {
			var v reflect.Value
			n, err := strconv.ParseUint(r.URL.Query().Get(":"+paramName), 10, 16)
			if err == nil {
				v := reflect.New(uint16Type).Elem()
				v.SetUint(n)
			}
			return v, err
		}
	case reflect.Uint32:
		return func(r *http.Request) (reflect.Value, error) {
			var v reflect.Value
			n, err := strconv.ParseUint(r.URL.Query().Get(":"+paramName), 10, 32)
			if err == nil {
				v := reflect.New(uint32Type).Elem()
				v.SetUint(n)
			}
			return v, err
		}
	case reflect.Uint64:
		return func(r *http.Request) (reflect.Value, error) {
			var v reflect.Value
			n, err := strconv.ParseUint(r.URL.Query().Get(":"+paramName), 10, 64)
			if err == nil {
				v := reflect.New(uint64Type).Elem()
				v.SetUint(n)
			}
			return v, err
		}
	}
	panic("unsupported path parameter type " + pt.String() + " for parameter " + paramName)
}
