package main

import (
	"fmt"
	"log"
	"time"

	"github.com/architectcgz/zhi-id-gen-go/pkg/client"
)

func main() {
	c := client.New(client.Config{
		ServerURL:       "http://localhost:8088",
		BufferEnabled:   true,
		BufferSize:      100,
		RefillThreshold: 20,
		BatchFetchSize:  50,
		AsyncRefill:     true,
		ReadTimeout:     3 * time.Second,
		ConnectTimeout:  3 * time.Second,
	})
	defer c.Close()

	snowflakeID, err := c.NextSnowflakeID()
	if err != nil {
		log.Fatalf("fetch snowflake id: %v", err)
	}

	orderID, err := c.NextSegmentID("order")
	if err != nil {
		log.Fatalf("fetch segment id: %v", err)
	}

	parsed, err := c.ParseSnowflakeID(snowflakeID)
	if err != nil {
		log.Fatalf("parse snowflake id: %v", err)
	}

	fmt.Printf("snowflake=%d\n", snowflakeID)
	fmt.Printf("segment(order)=%d\n", orderID)
	fmt.Printf("parsed worker=%d datacenter=%d sequence=%d\n", parsed.WorkerID, parsed.DatacenterID, parsed.Sequence)
}
