package main

import (
	"log"
	"net/http"
	"net/url"
	"time"

	"github.com/claytonseraphin/agent-observer/internal/api"
	"github.com/claytonseraphin/agent-observer/internal/grouping"
	"github.com/claytonseraphin/agent-observer/internal/proxy"
	"github.com/claytonseraphin/agent-observer/internal/tracker"
)

func main() {
	tr := tracker.NewTracker()
	asg := grouping.NewGroupAssigner(10 * time.Second)
	startCleanup(time.Minute, time.Hour, tr, asg)

	// Temporary: replaced by the per-project agent registry in Phase 4.
	routes := map[string]*url.URL{
		"news":      mustParse("http://localhost:8001"),
		"financial": mustParse("http://localhost:8002"),
	}

	mux := http.NewServeMux()
	api.Register(mux, tr)
	mux.Handle("/", proxy.New(tr, asg, routes, "http://localhost:8080"))

	log.Fatal(http.ListenAndServe(":8080", mux))
}
