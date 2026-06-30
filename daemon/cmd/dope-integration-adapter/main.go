// Command dope-integration-adapter is the integration adapter process for the capability RPC
// contract (Roadmap 59). By default it serves the reference skeleton (no real provider); set
// DOPE_ADAPTER_PROVIDER=feishu_lark to serve the real Feishu/Lark provider for a domain
// selected by DOPE_ADAPTER_DOMAIN (default "calendar", Roadmap 60).
//
// Reference mode honors DOPE_ADAPTER_FAIL (auth|malformed|hang|crash) to seed a failure mode
// for conformance and failure-isolation testing.
package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/dopejs/dope-agent/daemon/internal/integrations/adapterprovider"
	"github.com/dopejs/dope-agent/daemon/internal/integrations/adapterref"
	"github.com/dopejs/dope-agent/daemon/internal/integrations/providers/feishulark"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "dope-integration-adapter:", err)
		os.Exit(1)
	}
}

func run() error {
	switch strings.ToLower(strings.TrimSpace(os.Getenv("DOPE_ADAPTER_PROVIDER"))) {
	case "feishu_lark", "feishu", "lark":
		return runFeishuLark()
	default:
		opts := adapterref.Options{FailMode: adapterref.FailMode(os.Getenv("DOPE_ADAPTER_FAIL"))}
		return adapterref.ServeWithOptions(os.Stdin, os.Stdout, opts)
	}
}

func runFeishuLark() error {
	domain := strings.ToLower(strings.TrimSpace(os.Getenv("DOPE_ADAPTER_DOMAIN")))
	if domain == "" {
		domain = "calendar"
	}
	client := feishulark.NewClient(os.Getenv("DOPE_FEISHU_BASE_URL"), nil)
	switch domain {
	case "calendar":
		return adapterprovider.Serve(os.Stdin, os.Stdout, feishulark.NewCalendarProvider(client))
	default:
		return fmt.Errorf("feishu_lark provider does not serve domain %q in this roadmap", domain)
	}
}
