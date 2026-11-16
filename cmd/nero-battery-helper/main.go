// NERO Battery modifications, 2026-11-16. Distributed under GPL v2; see docs/CHANGES.md.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/charlie0129/batt/pkg/daemon"
	"github.com/charlie0129/batt/pkg/powerinfo"
	"github.com/charlie0129/batt/pkg/smc"
	"log"
	"os"
	"os/signal"
	"syscall"
)

func main() {
	if len(os.Args) == 2 && os.Args[1] == "--restore-power" {
		if os.Geteuid() != 0 {
			log.Fatal("power handback requires administrator authorization")
		}
		s := smc.New()
		if err := s.Open(); err != nil {
			log.Fatal(err)
		}
		defer s.Close()
		if err := errors.Join(s.EnableAdapter(), s.EnableCharging()); err != nil {
			log.Fatal(err)
		}
		adapter, aerr := s.IsAdapterEnabled()
		charging, cerr := s.IsChargingEnabled()
		if aerr != nil || cerr != nil || !adapter || !charging {
			log.Fatal("power handback could not be verified; keep the installed recovery files")
		}
		fmt.Println("External power and charging control returned to macOS.")
		return
	}
	if len(os.Args) == 2 && os.Args[1] == "--probe" {
		if _, err := powerinfo.ReadObservation(); err != nil {
			log.Fatal("a readable built-in battery is required: ", err)
		}
		s := smc.New()
		if err := s.Open(); err != nil {
			log.Fatal(err)
		}
		defer s.Close()
		json.NewEncoder(os.Stdout).Encode(map[string]any{"read_only": true, "charge_control_mode": s.ChargeControlMode(), "adapter_control": s.IsAdapterControlCapable()})
		return
	}
	if len(os.Args) != 1 {
		log.Fatal("unknown helper option; use --probe or --restore-power, or no arguments for the installed service")
	}
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	if err := daemon.RunLocalControl(ctx); err != nil {
		log.Fatal(err)
	}
}
