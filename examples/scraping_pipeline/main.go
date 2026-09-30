// Package main is a small scraping pipeline: target list → agent session
// per-target → a task that opens the page, takes a screenshot and reads the
// title → collect outputs.
//
// Demonstrates the resource composition pattern customers use most
// often (one session per workflow unit). Pairs well with goroutine_pool
// when scaling.
//
// Every agent session goes out through one of your saved proxies, so each
// create names its id. On hosted Driftstack only agent sessions load pages,
// so this pipeline uses client.AgentSessions, not the direct client.Sessions.
//
// Run:
//
//	DRIFTSTACK_API_KEY=ds_live_… DRIFTSTACK_PROXY_ID=<proxy id> go run ./examples/scraping_pipeline
package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	driftstack "github.com/driftstackdev/driftstack-go"
)

func main() {
	apiKey := os.Getenv("DRIFTSTACK_API_KEY")
	proxyID := os.Getenv("DRIFTSTACK_PROXY_ID")
	if apiKey == "" || proxyID == "" {
		log.Fatal("DRIFTSTACK_API_KEY and DRIFTSTACK_PROXY_ID required")
	}

	outDir := os.Getenv("OUT_DIR")
	if outDir == "" {
		outDir = "./scrape-output"
	}
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		log.Fatal(err)
	}

	client := driftstack.New(apiKey)
	defer client.Close()

	targets := []struct {
		name string
		url  string
	}{
		{"example", "https://example.com/"},
		{"go", "https://go.dev/"},
	}

	for _, target := range targets {
		if err := scrape(client, proxyID, target.name, target.url, outDir); err != nil {
			log.Printf("[%s] failed: %v", target.name, err)
			continue
		}
		log.Printf("[%s] ok", target.name)
	}
}

func scrape(client *driftstack.Client, proxyID, name, url, outDir string) error {
	// An AI turn plans and runs several steps, so give it minutes, not seconds.
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	agent, err := client.AgentSessions.Create(ctx, &driftstack.CreateAgentSessionRequest{
		Mode:    "ai",
		ProxyID: proxyID,
	}, nil)
	if err != nil {
		return fmt.Errorf("create: %w", err)
	}
	// Always close: an open session counts against your plan's concurrent
	// limit. Closing is idempotent.
	defer func() {
		_ = client.AgentSessions.Close(context.Background(), agent.ID)
	}()

	// A new session is "provisioning" until its browser is ready.
	for {
		s, err := client.AgentSessions.Get(ctx, agent.ID)
		if err != nil {
			return fmt.Errorf("get: %w", err)
		}
		if s.Status != "provisioning" {
			break
		}
		time.Sleep(2 * time.Second)
	}

	reply, err := client.AgentSessions.Message(ctx, agent.ID,
		fmt.Sprintf("Open %s, take a screenshot, and tell me the page title.", url),
		&driftstack.MessageOptions{IdempotencyKey: fmt.Sprintf("scrape-%s-%d", name, time.Now().UnixNano())})
	if err != nil {
		return fmt.Errorf("message: %w", err)
	}
	if reply.Kind != "plan-executed" {
		return fmt.Errorf("the agent did not run the task (kind %q)", reply.Kind)
	}
	if err := os.WriteFile(filepath.Join(outDir, name+".txt"), []byte(reply.Answer+"\n"), 0o644); err != nil {
		return fmt.Errorf("write answer: %w", err)
	}

	// A "capture" step's result carries a CaptureID; GetCapture returns the
	// image behind it.
	results, err := reply.ParsedResults()
	if err != nil {
		return fmt.Errorf("results: %w", err)
	}
	for _, r := range results {
		if r.Kind != "success" || r.CaptureID == "" {
			continue
		}
		shot, err := client.AgentSessions.GetCapture(ctx, agent.ID, r.CaptureID)
		if err != nil {
			return fmt.Errorf("capture: %w", err)
		}
		ext := ".png"
		if shot.ContentType == "image/jpeg" {
			ext = ".jpg"
		}
		path := filepath.Join(outDir, name+ext)
		if err := os.WriteFile(path, shot.Bytes, 0o644); err != nil {
			return fmt.Errorf("write: %w", err)
		}
		return nil
	}
	return fmt.Errorf("the agent took no screenshot")
}
