package main

import (
	"flag"
	"fmt"
	"time"
)

func main() {
	member := flag.String("member", "unknown", "gang member identifier")
	hold := flag.Duration("hold", 0, "time to remain running after reporting readiness")
	flag.Parse()
	fmt.Printf("member=%s started=%s\n", *member, time.Now().UTC().Format(time.RFC3339Nano))
	if *hold > 0 {
		time.Sleep(*hold)
	}
}
