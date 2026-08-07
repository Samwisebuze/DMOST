package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/samwisebuze/dmost/pkg/http"
)

// healthcheckAddr is where the probe looks for the daemon. It mirrors the
// listen address hard-coded in http.NewServer(); when that becomes configurable
// this should follow it.
const healthcheckAddr = "http://127.0.0.1:8080"

// healthcheckTimeout bounds the probe so it can never outlive the interval an
// orchestrator polls it on.
const healthcheckTimeout = 2 * time.Second

// runHealthcheck probes a daemon that is already running in this container and
// exits: 0 if it reports healthy, 1 otherwise.
//
// This exists so the release image can be probed at all. It is built FROM
// scratch and holds exactly one file, so there is no shell, no curl and no wget
// for a HEALTHCHECK to call — the binary has to be its own client. Docker's exec
// form invokes it directly rather than through /bin/sh, which is what makes that
// work:
//
//	HEALTHCHECK CMD ["/dmostd", "-healthcheck"]
func runHealthcheck() {
	ctx, cancel := context.WithTimeout(context.Background(), healthcheckTimeout)
	defer cancel()

	client := http.NewClient(http.WithServer(healthcheckAddr))
	if err := client.Health(ctx); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	os.Exit(0)
}
