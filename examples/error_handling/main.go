// Package main shows the typed-error catch patterns: errors.As for
// payload, errors.Is for category.
//
// Run:
//
//	DRIFTSTACK_API_KEY=ds_live_… DRIFTSTACK_PROXY_ID=<proxy id> go run ./examples/error_handling
package main

import (
	"context"
	"errors"
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
		log.Fatal("DRIFTSTACK_API_KEY and DRIFTSTACK_PROXY_ID required")
	}
	client := driftstack.New(apiKey)
	defer client.Close()
	ctx := context.Background()

	// Custom retry loop on top of the SDK's default policy. Most callers
	// don't need this — the SDK retries TransportError + RateLimitError
	// automatically. Shown here as a recipe for finer control. Every attempt
	// sends the same Idempotency-Key, so a retry never starts a second session.
	idemKey := fmt.Sprintf("error-handling-%d", time.Now().UnixNano())
	var session *driftstack.AgentSession
	for attempt := 0; attempt < 5; attempt++ {
		s, err := client.AgentSessions.Create(ctx,
			&driftstack.CreateAgentSessionRequest{Mode: "ai", ProxyID: proxyID},
			&driftstack.CreateOptions{IdempotencyKey: idemKey})
		if err == nil {
			session = s
			break
		}

		var rl *driftstack.RateLimitError
		if errors.As(err, &rl) {
			wait := time.Duration(rl.RetryAfterSeconds) * time.Second
			if wait == 0 {
				wait = time.Duration(1<<attempt) * time.Second
			}
			fmt.Printf("rate limited; waiting %v before retry %d/5\n", wait, attempt+1)
			time.Sleep(wait)
			continue
		}

		var cle *driftstack.ConcurrencyLimitError
		if errors.As(err, &cle) {
			log.Fatalf("concurrent-session ceiling: %d/%d", cle.CurrentSessions, cle.Limit)
		}

		var qe *driftstack.QuotaExceededError
		if errors.As(err, &qe) {
			log.Fatalf("quota exhausted for %s: %d/%d", qe.RecordType, qe.Current, qe.Limit)
		}

		// Sentinel-based catch-all for category.
		if errors.Is(err, driftstack.ErrAuth) {
			log.Fatalf("auth failure: %v", err)
		}

		log.Fatalf("create agent session: %v", err)
	}

	if session == nil {
		log.Fatal("gave up after 5 retries")
	}

	if err := client.AgentSessions.Close(ctx, session.ID); err != nil {
		log.Printf("warn: close failed: %v", err)
	}
}
