// Command dope-integration-adapter is the reference integration adapter skeleton (Roadmap
// 59). It serves the capability RPC contract over stdio with no real provider. Set
// DOPE_ADAPTER_FAIL (auth|malformed|hang|crash) to seed a failure mode for conformance and
// failure-isolation testing. Real providers replace it in Roadmap 60/63.
package main

import (
	"fmt"
	"os"

	"github.com/dopejs/dope-agent/daemon/internal/integrations/adapterref"
)

func main() {
	opts := adapterref.Options{FailMode: adapterref.FailMode(os.Getenv("DOPE_ADAPTER_FAIL"))}
	if err := adapterref.ServeWithOptions(os.Stdin, os.Stdout, opts); err != nil {
		fmt.Fprintln(os.Stderr, "dope-integration-adapter:", err)
		os.Exit(1)
	}
}
