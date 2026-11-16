// NERO Battery modifications, 2026-11-16. Distributed under GPL v2; see docs/CHANGES.md.
package main

import (
	"bufio"
	"encoding/json"
	"errors"
	"github.com/charlie0129/batt/pkg/localipc"
	"io"
	"os/exec"
	"sync"
	"sync/atomic"
	"time"
)

var control struct {
	sync.Mutex
	cmd    *exec.Cmd
	input  io.WriteCloser
	output *bufio.Reader
}

var controlClientActive atomic.Bool

// UI callbacks never wait for an in-flight request on the AppKit thread.
func setControlClientActive(active bool) {
	wasActive := controlClientActive.Swap(active)
	if active || !wasActive {
		return
	}
	go func() {
		control.Lock()
		defer control.Unlock()
		if !controlClientActive.Load() {
			releaseControlClient()
		}
	}()
}

// Called under control's mutex, after all request/reply work has finished.
// EOF normally lets the stateless client exit; the deadline bounds exit faults.
func releaseControlClient() {
	if control.cmd == nil {
		return
	}
	cmd := control.cmd
	_ = control.input.Close()
	done := make(chan struct{})
	go func() { _ = cmd.Wait(); close(done) }()
	timer := time.NewTimer(time.Second)
	defer timer.Stop()
	select {
	case <-done:
	case <-timer.C:
		_ = cmd.Process.Kill()
		<-done
	}
	control.cmd = nil
	control.input = nil
	control.output = nil
}

func closeControlClient() { control.Lock(); defer control.Unlock(); stopControlClient() }
func stopControlClient() {
	if control.cmd != nil {
		_ = control.input.Close()
		_ = control.cmd.Process.Kill()
		_ = control.cmd.Wait()
	}
	control.cmd = nil
	control.input = nil
	control.output = nil
}

func controlRequest(command, payload string) ([]byte, error) {
	control.Lock()
	defer func() {
		if !controlClientActive.Load() {
			releaseControlClient()
		}
		control.Unlock()
	}()
	if len(payload) > 16*1024 {
		return nil, errors.New("control payload exceeds limit")
	}
	if control.cmd == nil {
		p, err := localipc.LoadPolicy(localipc.DefaultPolicyPath)
		if err != nil {
			return nil, err
		}
		if err = p.VerifyControlClient(); err != nil {
			return nil, err
		}
		cmd := exec.Command(localipc.ControlClientPath)
		cmd.Env = []string{"PATH=/usr/bin:/bin:/usr/sbin:/sbin", "LC_ALL=C"}
		input, err := cmd.StdinPipe()
		if err != nil {
			return nil, err
		}
		output, err := cmd.StdoutPipe()
		if err != nil {
			input.Close()
			return nil, err
		}
		if err = cmd.Start(); err != nil {
			input.Close()
			output.Close()
			return nil, err
		}
		control.cmd = cmd
		control.input = input
		control.output = bufio.NewReader(output)
	}
	request, _ := json.Marshal(localipc.ControlRequest{Command: command, Payload: payload})
	if _, err := control.input.Write(append(request, '\n')); err != nil {
		stopControlClient()
		return nil, err
	}
	type result struct {
		data []byte
		err  error
	}
	done := make(chan result, 1)
	reader := control.output
	go func() {
		var line []byte
		for {
			part, err := reader.ReadSlice('\n')
			line = append(line, part...)
			if len(line) > 3*1024*1024 {
				done <- result{nil, errors.New("control reply exceeds limit")}
				return
			}
			if err == bufio.ErrBufferFull {
				continue
			}
			done <- result{line, err}
			return
		}
	}()
	timer := time.NewTimer(15 * time.Second)
	defer timer.Stop()
	select {
	case r := <-done:
		if r.err != nil {
			stopControlClient()
			return nil, r.err
		}
		var reply localipc.ControlReply
		if err := json.Unmarshal(r.data, &reply); err != nil {
			stopControlClient()
			return nil, err
		}
		if reply.Error != "" {
			return nil, errors.New(reply.Error)
		}
		return reply.Body, nil
	case <-timer.C:
		stopControlClient()
		return nil, errors.New("control request timed out; operation result unknown; not retried")
	}
}
