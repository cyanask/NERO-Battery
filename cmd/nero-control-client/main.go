// NERO Battery modifications, 2026-11-16. Distributed under GPL v2; see docs/CHANGES.md.
package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"github.com/charlie0129/batt/pkg/localipc"
	"github.com/charlie0129/batt/pkg/localxpc"
	"os"
)

func main() {
	p, err := localipc.LoadPolicy(localipc.DefaultPolicyPath)
	if err == nil {
		err = p.VerifyControlClient()
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	scan := bufio.NewScanner(os.Stdin)
	scan.Buffer(make([]byte, 4096), 128*1024)
	out := json.NewEncoder(os.Stdout)
	for scan.Scan() {
		var request localipc.ControlRequest
		reply := localipc.ControlReply{}
		if err := json.Unmarshal(scan.Bytes(), &request); err != nil || len(request.Payload) > 16*1024 {
			reply.Error = "invalid control request"
		} else if _, _, ok := localxpc.Endpoint(request.Command); !ok {
			reply.Error = "unsupported control command"
		} else {
			body, err := localxpc.Request(request.Command, []byte(request.Payload))
			if err != nil {
				reply.Error = err.Error()
			} else {
				reply.Body = body
			}
		}
		if out.Encode(reply) != nil {
			return
		}
	}
	if err := scan.Err(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
