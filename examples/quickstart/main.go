// Package main is the quickstart example for the Driftstack Go SDK: run an
// agent session — create it through one of your saved proxies, give the AI a
// task, print its answer, close it.
//
// Run:
//
//	DRIFTSTACK_API_KEY=ds_live_… DRIFTSTACK_PROXY_ID=<proxy id> go run ./examples/quickstart
//
// Every agent session goes out through one of your saved proxies. List their
// ids with GET /v1/account/me/proxies, using a key with the account_owner scope.
package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	driftstack "github.com/driftstackdev/driftstack-go"
)

func main() {
	apiKey := os.Getenv("DRIFTSTACK_API_KEY")
	proxyID := os.Getenv("DRIFTSTACK_PROXY_ID")
	if apiKey == "" || proxyID == "" {
		log.Fatal("DRIFTSTACK_API_KEY and DRIFTSTACK_PROXY_ID environment variables are required")
	}

	opts := []driftstack.Option{}
	if base := os.Getenv("DRIFTSTACK_BASE_URL"); base != "" {
		opts = append(opts, driftstack.WithBaseURL(base))
	}
	client := driftstack.New(apiKey, opts...)
	defer client.Close()

	ctx := context.Background()

	agent, err := client.AgentSessions.Create(ctx, &driftstack.CreateAgentSessionRequest{
		Mode:    "ai",
		ProxyID: proxyID,
	}, &driftstack.CreateOptions{IdempotencyKey: fmt.Sprintf("quickstart-%d", time.Now().UnixNano())})
	if err != nil {
		log.Fatalf("create agent session: %v", err)
	}
	fmt.Printf("created agent session %s\n", agent.ID)
	// Always close: an open session counts against your plan's concurrent
	// limit. Closing is idempotent.
	defer func() {
		if err := client.AgentSessions.Close(context.Background(), agent.ID); err != nil {
			log.Printf("close: %v", err)
			return
		}
		fmt.Printf("closed agent session %s\n", agent.ID)
	}()

	// A new session is "provisioning" until its browser is ready.
	for {
		s, err := client.AgentSessions.Get(ctx, agent.ID)
		if err != nil {
			log.Printf("get: %v", err)
			return
		}
		if s.Status != "provisioning" {
			break
		}
		time.Sleep(2 * time.Second)
	}

	reply, err := client.AgentSessions.Message(ctx, agent.ID,
		"Open https://example.com and tell me the page title.",
		&driftstack.MessageOptions{IdempotencyKey: fmt.Sprintf("quickstart-task-%d", time.Now().UnixNano())})
	if err != nil {
		log.Printf("message: %v", err)
		return
	}
	fmt.Println(reply.Kind, reply.Answer)
}
