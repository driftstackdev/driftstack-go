// Package driftstack is the official Go SDK for the Driftstack API —
// iPhone Safari browser automation, called from Go.
//
// Quickstart: run steps you write on an agent session. On hosted Driftstack
// every session goes out through one of your saved proxies, so the create
// names one (list their ids with client.Egress.ListProxies, using a key with
// the account_owner scope); a create without one is refused with a 422
// *ProxyRequiredError. The step-by-step operations of client.Sessions cannot
// run on a hosted session yet (each answers 503), so drive the page here.
//
//	client := driftstack.New("ds_live_…")
//	defer client.Close()
//
//	ctx := context.Background()
//	agent, err := client.AgentSessions.Create(ctx, &driftstack.CreateAgentSessionRequest{
//		ProxyID: "<proxy id>",
//	}, nil)
//	if err != nil {
//		log.Fatal(err)
//	}
//	// An open session counts against your plan's limit: close it. Past this
//	// point, return on an error rather than log.Fatal, so this runs.
//	defer client.AgentSessions.Close(context.Background(), agent.ID)
//
//	for !agent.IsReady() {
//		if agent.Status == "closed" {
//			log.Print("the session closed before its browser was ready")
//			return
//		}
//		time.Sleep(2 * time.Second)
//		if agent, err = client.AgentSessions.Get(ctx, agent.ID); err != nil {
//			log.Print(err)
//			return
//		}
//	}
//	run, err := client.AgentSessions.RunSteps(ctx, agent.ID, []driftstack.AgentIntent{
//		{Kind: "navigate", URL: "https://example.com/"},
//		{Kind: "extract", Selector: "h1"},
//	}, nil)
//	if err != nil {
//		log.Print(err)
//		return
//	}
//	results, _ := run.ParsedResults()
//	for _, r := range results {
//		log.Printf("%s %s: %s", r.Intent.Kind, r.Kind, r.Summary)
//	}
//
// To give the AI a task in plain words instead, send it with
// client.AgentSessions.Message.
//
// Errors are typed: every server problem-type maps to a concrete error
// type customers can switch on with errors.As. The retry policy is
// applied automatically (configurable via [WithRetry]) and honours
// Retry-After. Retries fire on transport errors, on 429 rate limits,
// and on InternalError — the plain 500. Every other typed error is
// terminal, including the other 5xx kinds such as DriverError (502),
// where retrying an idempotent call would not help. That is the same
// set the TypeScript and Python SDKs retry; [IsRetryable] is the
// exported predicate, and the loop uses it.
//
// Because a transport error can mean a request the server already
// processed but whose response was lost, an automatically-retried create
// or charge can execute twice — pass an IdempotencyKey on those calls
// (e.g. CreateOptions.IdempotencyKey) so the server collapses the retry.
//
// Webhook signature verification is in [VerifyWebhookSignature].
//
// Module path: github.com/driftstackdev/driftstack-go
package driftstack
