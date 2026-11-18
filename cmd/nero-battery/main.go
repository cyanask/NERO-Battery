// NERO Battery modifications, 2026-11-16. Distributed under GPL v2; see docs/CHANGES.md.
package main

/*
#cgo CFLAGS: -x objective-c -fobjc-arc
#cgo LDFLAGS: -framework Cocoa
#include <stdlib.h>
void neroRun(void);
char *neroUSBPowerJSON(void);
void neroRenderJSON(const char *json, int connected);
void neroActionFinished(int action, int ok, const char *message);
void neroHistoryJSON(const char *json);
*/
import "C"

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"unsafe"

	"github.com/charlie0129/batt/pkg/localipc"
	"github.com/charlie0129/batt/pkg/localxpc"
	"github.com/charlie0129/batt/pkg/powerinfo"
)

var refreshing atomic.Bool
var actions sync.Mutex

//export neroControlClientActive
func neroControlClientActive(active C.int) { setControlClientActive(active != 0) }

func request(method, path, body string) ([]byte, error) {
	command, err := localxpc.Command(method, path)
	if err != nil {
		return nil, err
	}
	return controlRequest(command, body)
}

func sendAction(action int, body string) ([]byte, error) {
	method, path := "POST", ""
	switch action {
	case 1:
		method, path = "PUT", "/band"
	case 2:
		path = "/charge-once/full"
	case 3:
		path = "/charge-once/cancel"
	case 4:
		path = "/discharge/start"
	case 5:
		path = "/discharge/stop"
	case 10:
		method, path = "PUT", "/thermal"
	case 11:
		path = "/calibration/start"
	case 12:
		path = "/calibration/pause"
	case 13:
		path = "/calibration/resume"
	case 14:
		path = "/calibration/cancel"
	case 20:
		method, path = "PUT", "/schedules"
	case 21:
		method, path = "PUT", "/power-mode"
	case 22:
		path = "/energy/sample"
	case 40:
		method, path = "PUT", "/sleep-policy"
	case 41:
		path = "/awake/start"
	case 42:
		path = "/awake/stop"
	case 23:
		method, path = "GET", "/power-mode"
	default:
		return nil, errors.New("unknown local action")
	}
	return request(method, path, body)
}

func main() {
	defer closeControlClient()
	if len(os.Args) == 2 && os.Args[1] == "--usb-power" {
		p := C.neroUSBPowerJSON()
		fmt.Println(C.GoString(p))
		C.free(unsafe.Pointer(p))
		return
	}
	if len(os.Args) > 1 && os.Args[1] == "--status" {
		runCLIStatus()
		return
	}
	if len(os.Args) > 1 && os.Args[1] == "--action" {
		runCLIAction()
		return
	}
	runtime.LockOSThread()
	C.neroRun()
}

func runCLIStatus() {
	b, err := request("GET", "/status", "")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	var fields map[string]any
	if err := json.Unmarshal(b, &fields); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	network, err := localipc.OwnSocketCounts()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fields["client_network"] = network
	fields["control_transport"] = "NSXPCConnection"
	encoded, _ := json.MarshalIndent(fields, "", "  ")
	fmt.Println(string(encoded))
}

func runCLIAction() {
	flags := flag.NewFlagSet("NERO Battery", flag.ContinueOnError)
	action := flags.String("action", "", "status, limit, full, cancel-full, discharge, stop-discharge, calibrate, pause-calibration, resume-calibration, cancel-calibration, power-mode")
	value := flags.String("value", "", "limit percentage or power mode")
	if err := flags.Parse(os.Args[1:]); err != nil {
		os.Exit(2)
	}
	if *action == "status" {
		runCLIStatus()
		return
	}
	var b []byte
	var err error
	switch *action {
	case "limit":
		n, e := strconv.Atoi(*value)
		if e != nil {
			err = e
		} else {
			b, err = request("PUT", "/limit", strconv.Itoa(n))
		}
	case "awake":
		n, e := strconv.Atoi(*value)
		if e != nil {
			err = e
		} else {
			b, err = sendAction(41, strconv.Itoa(n))
		}
	case "sleep":
		b, err = sendAction(42, "")
	case "full":
		b, err = sendAction(2, "")
	case "cancel-full":
		b, err = sendAction(3, "")
	case "discharge":
		b, err = sendAction(4, "")
	case "stop-discharge":
		b, err = sendAction(5, "")
	case "calibrate":
		b, err = sendAction(11, `{"threshold":20,"hold_minutes":60}`)
	case "pause-calibration":
		b, err = sendAction(12, "")
	case "resume-calibration":
		b, err = sendAction(13, "")
	case "cancel-calibration":
		b, err = sendAction(14, "")
	case "power-mode":
		payload, _ := json.Marshal(*value)
		b, err = sendAction(21, string(payload))
	default:
		err = errors.New("unknown action; no hardware operation performed")
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Println(string(b))
}

//export neroRefresh
func neroRefresh() {
	if !refreshing.CompareAndSwap(false, true) {
		return
	}
	go func() {
		defer refreshing.Store(false)
		b, err := request("GET", "/status", "")
		connected := 1
		if err != nil {
			o, _ := powerinfo.ReadObservation()
			b, _ = json.Marshal(map[string]any{"observation": o})
			connected = 0
		}
		p := C.CString(string(b))
		C.neroRenderJSON(p, C.int(connected))
		C.free(unsafe.Pointer(p))
	}()
}

//export neroPerform
func neroPerform(action C.int, payload *C.char) {
	body := C.GoString(payload)
	go func() {
		actions.Lock()
		defer actions.Unlock()
		if int(action) == 30 {
			if body != "1h" && body != "24h" && body != "7d" && body != "30d" {
				return
			}
			b, err := request("GET", "/history?range="+body, "")
			if err == nil {
				p := C.CString(string(b))
				C.neroHistoryJSON(p)
				C.free(unsafe.Pointer(p))
				return
			}
			finish(int(action), false, "历史读取未完成，请稍后重试。")
			return
		}
		_, err := sendAction(int(action), body)
		message := "已完成。"
		if err != nil {
			message = actionError(err)
		}
		finish(int(action), err == nil, message)
		neroRefresh()
	}()
}

func finish(action int, ok bool, message string) {
	success := 0
	if ok {
		success = 1
	}
	p := C.CString(message)
	C.neroActionFinished(C.int(action), C.int(success), p)
	C.free(unsafe.Pointer(p))
}

func actionError(err error) string {
	e := err.Error()
	switch {
	case strings.HasPrefix(e, "limit saved, but protection"):
		return "充电范围已保存；当前保护规则暂停执行，条件恢复后再应用。"
	case strings.HasPrefix(e, "limit saved, but"):
		return "充电范围已保存，硬件状态尚未确认；请查看最新状态。"
	case strings.Contains(e, "safety data"):
		return "保护数据未更新，受控充电保持暂停。"
	case strings.Contains(e, "sleep protection"):
		return "睡眠或合盖保护正在暂停充电控制。"
	case strings.Contains(e, "XPC"):
		return "后台连接中断，操作结果待确认；未自动重复执行。"
	case strings.Contains(e, "AlDente"):
		return "AlDente 尚未完全退出，已拒绝切换。"
	case strings.Contains(e, "thermal") && !strings.Contains(e, "thermal limits"):
		return "热保护已暂停充电与放电，请先查看温度与保护状态。"
	case strings.Contains(e, "30%"):
		return "电量须高于 30% 才能开始手动放电。"
	case strings.Contains(e, "already at"):
		return "当前电量已达到目标，无需继续充电。"
	case strings.Contains(e, "lid"):
		return "请先打开电脑屏幕。"
	case strings.Contains(e, "connect external"):
		return "请先连接外接电源。"
	case strings.Contains(e, "calibration"):
		return "校准状态限制了此操作，请先暂停或取消校准，并查看状态说明。"
	case strings.Contains(e, "one-time charge"):
		return "请先取消临时充满，再开始放电。"
	case strings.Contains(e, "adapter is disabled"), strings.Contains(e, "temporary power-adapter"):
		return "请先停止手动放电，再临时充满。"
	case strings.Contains(e, "hysteresis"), strings.Contains(e, "thermal limits"):
		return "暂停温度应为 35–45°C，恢复温度至少低 2°C。"
	case strings.Contains(e, "restart gap"):
		return "充电上限应为 50–100%，恢复间隔应为 2–10 个百分点。"
	default:
		return "操作结果尚未确认，请查看最新状态后再决定是否重试。"
	}
}
