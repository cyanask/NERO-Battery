// NERO Battery modifications, 2026-11-16. Distributed under GPL v2; see docs/CHANGES.md.
// Package localxpc provides the fixed NSXPC Mach-service control channel.
package localxpc

/*
#cgo CFLAGS: -x objective-c -fobjc-arc
#cgo LDFLAGS: -framework Foundation
#include <stdlib.h>
#include "native.h"
*/
import "C"
import (
	"errors"
	"github.com/charlie0129/batt/pkg/localipc"
	"strings"
	"sync"
	"unsafe"
)

const ServiceName = "io.nero.battery.helper"
const maxPayload = 16 * 1024
const maxReply = 2 * 1024 * 1024

// This fixed command vocabulary is the XPC API. No caller-supplied URL or method.
var endpoints = map[string][2]string{
	"status": {"GET", "/status"}, "band": {"PUT", "/band"}, "limit": {"PUT", "/limit"},
	"full": {"POST", "/charge-once/full"}, "cancel-full": {"POST", "/charge-once/cancel"},
	"discharge": {"POST", "/discharge/start"}, "stop-discharge": {"POST", "/discharge/stop"},
	"thermal": {"PUT", "/thermal"}, "schedules": {"PUT", "/schedules"}, "power-get": {"GET", "/power-mode"}, "power-set": {"PUT", "/power-mode"}, "energy": {"POST", "/energy/sample"},
	"calibrate": {"POST", "/calibration/start"}, "pause-calibration": {"POST", "/calibration/pause"}, "resume-calibration": {"POST", "/calibration/resume"}, "cancel-calibration": {"POST", "/calibration/cancel"},
	"sleep-policy": {"PUT", "/sleep-policy"}, "awake": {"POST", "/awake/start"}, "allow-sleep": {"POST", "/awake/stop"},
	"history-1h": {"GET", "/history?range=1h"}, "history-24h": {"GET", "/history?range=24h"}, "history-7d": {"GET", "/history?range=7d"}, "history-30d": {"GET", "/history?range=30d"},
}

func Endpoint(command string) (method, path string, ok bool) {
	p, ok := endpoints[command]
	return p[0], p[1], ok
}
func Command(method, path string) (string, error) {
	for command, p := range endpoints {
		if p[0] == method && p[1] == path {
			return command, nil
		}
	}
	return "", errors.New("unsupported XPC command")
}
func Request(command string, payload []byte) ([]byte, error) {
	if _, _, ok := Endpoint(command); !ok {
		return nil, errors.New("unsupported XPC command")
	}
	if len(payload) > maxPayload {
		return nil, errors.New("XPC payload exceeds limit")
	}
	p, err := localipc.LoadPolicy(localipc.DefaultPolicyPath)
	if err != nil {
		return nil, err
	}
	_, server, _, err := p.XPCIdentity()
	if err != nil {
		return nil, err
	}
	req, cmd := C.CString(server), C.CString(command)
	defer C.free(unsafe.Pointer(req))
	defer C.free(unsafe.Pointer(cmd))
	data := C.CBytes(payload)
	defer C.free(data)
	var status C.int
	var length C.size_t
	var failure *C.char
	response := C.neroXPCRequest(req, cmd, data, C.size_t(len(payload)), &status, &length, &failure)
	if response != nil {
		defer C.free(response)
	}
	if failure != nil {
		defer C.free(unsafe.Pointer(failure))
		return nil, errors.New(C.GoString(failure))
	}
	if length > maxReply {
		return nil, errors.New("XPC reply exceeds limit")
	}
	body := C.GoBytes(response, C.int(length))
	if status < 200 || status >= 300 {
		return nil, errors.New(strings.TrimSpace(string(body)))
	}
	return body, nil
}

type Handler func(command string, payload []byte) (status int, body []byte)

var handlers struct {
	sync.RWMutex
	handler Handler
}

func Listen(p *localipc.Policy, h Handler) (func(), error) {
	if h == nil {
		return nil, errors.New("XPC handler required")
	}
	if err := p.VerifyXPCServer(); err != nil {
		return nil, err
	}
	uid, _, client, err := p.XPCIdentity()
	if err != nil {
		return nil, err
	}
	handlers.Lock()
	defer handlers.Unlock()
	if handlers.handler != nil {
		return nil, errors.New("XPC listener already running")
	}
	handlers.handler = h
	name := C.CString(client)
	defer C.free(unsafe.Pointer(name))
	var failure *C.char
	if C.neroXPCStart(C.uint(uid), name, &failure) == 0 {
		handlers.handler = nil
		if failure != nil {
			defer C.free(unsafe.Pointer(failure))
			return nil, errors.New(C.GoString(failure))
		}
		return nil, errors.New("XPC listener could not start")
	}
	var once sync.Once
	return func() {
		once.Do(func() { C.neroXPCStop(); handlers.Lock(); handlers.handler = nil; handlers.Unlock() })
	}, nil
}

//export neroXPCDispatch
func neroXPCDispatch(command *C.char, payload unsafe.Pointer, size C.size_t, status *C.int, body *unsafe.Pointer, length *C.size_t) {
	code, result := dispatch(C.GoString(command), payload, size)
	*status = C.int(code)
	*length = C.size_t(len(result))
	*body = C.CBytes(result)
}
func dispatch(command string, payload unsafe.Pointer, size C.size_t) (code int, body []byte) {
	defer func() {
		if recover() != nil {
			code = 500
			body = []byte("local command failed")
		}
	}()
	if size > maxPayload {
		return 413, []byte("XPC payload exceeds limit")
	}
	if _, _, ok := Endpoint(command); !ok {
		return 400, []byte("unsupported XPC command")
	}
	handlers.RLock()
	defer handlers.RUnlock()
	if handlers.handler == nil {
		return 503, []byte("control stopped")
	}
	code, body = handlers.handler(command, C.GoBytes(payload, C.int(size)))
	if len(body) > maxReply {
		return 413, []byte("XPC reply exceeds limit")
	}
	return code, body
}
