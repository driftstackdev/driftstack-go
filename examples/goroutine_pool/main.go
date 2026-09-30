// Package main shows a worker-pool pattern: fan out N concurrent
// session ops, collect results. Honours the SDK's tier rate limit
// because each worker uses the same client (which retries + bounded
// retries; rate-limit excursions automatically back off).
//
// Each job is one agent session: create it through one of your saved
// proxies, give the AI the page to read, close it. On hosted Driftstack only
// agent sessions load pages, so the workers use client.AgentSessions, not the
// direct client.Sessions. Keep numWorkers at or below your plan's
// concurrent-session limit: every open session counts against it.
//
// Run:
//
//	DRIFTSTACK_API_KEY=ds_live_… DRIFTSTACK_PROXY_ID=<proxy id> go run ./examples/goroutine_pool
package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"sync"
	"time"

	driftstack "github.com/driftstackdev/driftstack-go"
)

const numWorkers = 4

func main() {
	apiKey := os.Getenv("DRIFTSTACK_API_KEY")
	proxyID := os.Getenv("DRIFTSTACK_PROXY_ID")
	if apiKey == "" || proxyID == "" {
		log.Fatal("DRIFTSTACK_API_KEY and DRIFTSTACK_PROXY_ID required")
	}
	client := driftstack.New(apiKey)
	defer client.Close()
	ctx := context.Background()

	urls := []string{
		"https://example.com/",
		"https://example.org/",
		"https://example.net/",
		"https://golang.org/",
		"https://httpbin.org/get",
	}

	jobs := make(chan string, len(urls))
	results := make(chan string, len(urls))
	var wg sync.WaitGroup

	worker := func(id int) {
		defer wg.Done()
		for url := range jobs {
			results <- fmt.Sprintf("worker %d url %s %s", id, url, readTitle(ctx, client, proxyID, url))
		}
	}

	for i := 0; i < numWorkers; i++ {
		wg.Add(1)
		go worker(i)
	}
	for _, u := range urls {
		jobs <- u
	}
	close(jobs)

	go func() {
		wg.Wait()
		close(results)
	}()

	for r := range results {
		fmt.Println(r)
	}
}

// readTitle runs one agent session end to end and reports the page title
// the agent read back, or the step that failed.
func readTitle(ctx context.Context, client *driftstack.Client, proxyID, url string) string {
	agent, err := client.AgentSessions.Create(ctx, &driftstack.CreateAgentSessionRequest{
		Mode:    "ai",
		ProxyID: proxyID,
	}, nil)
	if err != nil {
		return fmt.Sprintf("ERR create: %v", err)
	}
	// Close on every path, success or error, so no session is left open.
	defer func() {
		_ = client.AgentSessions.Close(context.Background(), agent.ID)
	}()

	// A new session is "provisioning" until its browser is ready.
	for {
		s, err := client.AgentSessions.Get(ctx, agent.ID)
		if err != nil {
			return fmt.Sprintf("ERR get: %v", err)
		}
		if s.Status != "provisioning" {
			break
		}
		time.Sleep(2 * time.Second)
	}

	reply, err := client.AgentSessions.Message(ctx, agent.ID,
		fmt.Sprintf("Open %s and tell me the page title.", url),
		&driftstack.MessageOptions{IdempotencyKey: fmt.Sprintf("pool-%s-%d", agent.ID, time.Now().UnixNano())})
	if err != nil {
		return fmt.Sprintf("ERR message: %v", err)
	}
	if reply.Kind != "plan-executed" {
		return fmt.Sprintf("ERR kind=%s", reply.Kind)
	}
	return fmt.Sprintf("OK title=%q", reply.Answer)
}
