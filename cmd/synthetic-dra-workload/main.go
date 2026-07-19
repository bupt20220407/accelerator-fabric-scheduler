package main

import (
	"flag"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"
)

func main() {
	expected := flag.Int("expected", 4, "expected number of DRA device IDs")
	hold := flag.Duration("hold", 0, "time to remain running after reporting CDI metadata")
	flag.Parse()
	claimUID := os.Getenv("BUPT_DRA_CLAIM_UID")
	rawDeviceIDs := os.Getenv("BUPT_DRA_DEVICE_IDS")
	if claimUID == "" || rawDeviceIDs == "" {
		fmt.Fprintln(os.Stderr, "DRA CDI environment was not injected")
		os.Exit(1)
	}
	deviceIDs := strings.Split(rawDeviceIDs, ",")
	sort.Strings(deviceIDs)
	if len(deviceIDs) != *expected {
		fmt.Fprintf(os.Stderr, "received %d DRA devices, want %d: %s\n", len(deviceIDs), *expected, rawDeviceIDs)
		os.Exit(1)
	}
	fmt.Printf("claim=%s devices=%s\n", claimUID, strings.Join(deviceIDs, ","))
	if *hold > 0 {
		time.Sleep(*hold)
	}
}
