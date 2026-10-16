package proxy

import (
	"log"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
	"time"

	"github.com/claytonseraphin/agent-observer/internal/grouping"
	"github.com/claytonseraphin/agent-observer/internal/outcome"
	"github.com/claytonseraphin/agent-observer/internal/tracker"
)

// Proxy forwards requests to the agents and records every call it sees.
type Proxy struct {
	tr         *tracker.Tracker
	asg        *grouping.GroupAssigner
	routes     map[string]*url.URL
	publicBase string
}

func New(tr *tracker.Tracker, asg *grouping.GroupAssigner,
	routes map[string]*url.URL, publicBase string) *Proxy {
	return &Proxy{tr: tr, asg: asg, routes: routes, publicBase: publicBase}
}

func clientAddress(r *http.Request) string {
	// Assuming the client application is behind a proxy, load balancer or CDN
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		first, _, _ := strings.Cut(xff, ",")
		return strings.TrimSpace(first)
	}
	if realIP := r.Header.Get("X-Real-IP"); realIP != "" {
		return realIP
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

func (p *Proxy) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	log.Printf("incoming: %s %s", r.Method, r.URL.Path)

	name, rest, _ := strings.Cut(strings.TrimPrefix(r.URL.Path, "/"), "/")
	target, ok := p.routes[name]
	if !ok {
		http.NotFound(w, r)
		return
	}

	var groupID string
	callIndex := -1
	address := clientAddress(r)

	isCall := r.Method == http.MethodPost
	isDiscovery := r.Method == http.MethodGet && strings.HasSuffix(r.URL.Path, "agent-card.json")

	if isCall || isDiscovery {
		kind := "call"
		if isDiscovery {
			kind = "discovery"
		}

		origin := "client"
		groupID = r.Header.Get("X-Trace-Id")
		if groupID == "" {
			origin = "inferred"
			groupID = p.asg.Begin(address)
			defer p.asg.End(address)
		}
		callIndex = p.tr.StartCall(groupID, origin, name, kind)
		log.Printf("group=%s origin=%s agent=%s kind=%s", groupID, origin, name, kind)
	}

	start := time.Now()
	rp := &httputil.ReverseProxy{
		Rewrite: func(pr *httputil.ProxyRequest) {
			pr.SetURL(target)
			pr.Out.URL.Path = "/" + rest
			pr.Out.Header.Del("Accept-Encoding")
		},
		ModifyResponse: func(resp *http.Response) error {
			log.Printf("%s %s -> %s %d (%s)", r.Method, r.URL.Path, name,
				resp.StatusCode, time.Since(start))

			// Only tracked requests have a callIndex; everything else keeps -1.
			if callIndex >= 0 {
				p.tr.FinishCall(groupID, callIndex, outcome.OutcomeFromResponse(resp))
			}

			if strings.HasSuffix(r.URL.Path, "agent-card.json") &&
				resp.StatusCode == http.StatusOK &&
				strings.Contains(resp.Header.Get("Content-Type"), "json") {
				return rewriteCard(resp, target.String()+"/", p.publicBase+"/"+name+"/")
			}
			return nil
		},
		ErrorHandler: func(w http.ResponseWriter, req *http.Request, err error) {
			log.Printf("%s %s -> %s failed: %v", req.Method, req.URL.Path, name, err)
			if callIndex >= 0 {
				p.tr.FinishCall(groupID, callIndex, outcome.OutcomeFromError(err))
			}
			http.Error(w, "upstream error", http.StatusBadGateway)
		},
	}
	rp.ServeHTTP(w, r)
}
