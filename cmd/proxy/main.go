package main

import(
  "bytes"
  "io"
  "net/http"
  "net/http/httputil"
  "net/url"
  //"slices"
  "strings"
  "log"
  "strconv"
  "time"
  "sync"
  "encoding/json"
  "net"
)

const publicBase = "http://localhost:8080"

var routes = map[string]*url.URL{
  "news":           mustParse("http://localhost:8001"),
  "financial":      mustParse("http://localhost:8002"),
  "education":      mustParse("http://localhost:8003"),
}


type CallRecord struct {
  Agent        string
  Kind         string
  StartedAt    time.Time
  EndedAt      time.Time
  Status       string   // processing, success, failed
  Error        string
}

type GroupRecord struct {
  Origin       string   // Client or inferred
  Calls        []*CallRecord
}

type Tracker struct {
  mu          sync.Mutex
  groups     map[string]*GroupRecord
}


func (t *Tracker) StartCall(groupID, origin, agent, kind string) int {
	t.mu.Lock()
	defer t.mu.Unlock()

	group, ok := t.groups[groupID]
	if !ok {
		group = &GroupRecord{Origin: origin}
		t.groups[groupID] = group
	}

	call := &CallRecord{
		Agent:     agent,
    Kind:      kind,
		StartedAt: time.Now(),
		Status:    StatusProcessing,
	}
	group.Calls = append(group.Calls, call)

	return len(group.Calls) - 1
}

func (t *Tracker) FinishCall(groupID string, callIndex int, outcome CallOutcome) {
	t.mu.Lock()
	defer t.mu.Unlock()

	g, ok := t.groups[groupID]
	if !ok || callIndex < 0 || callIndex >= len(g.Calls) {
		return
	}
	call := g.Calls[callIndex]
	if call.Status != StatusProcessing {
		return // already finished, keep the first outcome
	}
	call.EndedAt = time.Now()
	call.Status = outcome.Status
	call.Error = outcome.Error
}

func NewTracker() *Tracker {
	return &Tracker{groups: map[string]*GroupRecord{}}
}

var tracker = NewTracker()

var assigner = NewGroupAssigner(10 * time.Second)

func mustParse(s string) *url.URL {
  u, err := url.Parse(s)
  if err != nil {
    log.Fatal(err)
  }
  return u
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
func handler(w http.ResponseWriter, r *http.Request) {
  log.Printf("incoming: %s %s", r.Method, r.URL.Path)

  name, rest, _ := strings.Cut(strings.TrimPrefix(r.URL.Path, "/"), "/")
  target, ok := routes[name]
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
  		groupID = assigner.Begin(address)
  		defer assigner.End(address)
  	}
  	callIndex = tracker.StartCall(groupID, origin, name, kind)
  	log.Printf("group=%s origin=%s agent=%s kind=%s", groupID, origin, name, kind)
  }

  start := time.Now()
  proxy := &httputil.ReverseProxy{
    Rewrite: func(pr *httputil.ProxyRequest) {
      pr.SetURL(target)
      pr.Out.URL.Path = "/" + rest
      //pr.Out.Host = target.Host
      pr.Out.Header.Del("Accept-Encoding")
    },
    ModifyResponse: func(resp *http.Response) error {
    	log.Printf("%s %s -> %s %d (%s)", r.Method, r.URL.Path, name,
    		resp.StatusCode, time.Since(start))

    	// Only POSTs have a callIndex. Card GETs keep -1 and are skipped.
    	if callIndex >= 0 {
    		tracker.FinishCall(groupID, callIndex, OutcomeFromResponse(resp))
    	}

    	if strings.HasSuffix(r.URL.Path, "agent-card.json") &&
    		resp.StatusCode == http.StatusOK &&
    		strings.Contains(resp.Header.Get("Content-Type"), "json") {
    		return rewriteCard(resp, target.String()+"/", publicBase+"/"+name+"/")
    	}
    	return nil
    },
    ErrorHandler: func(w http.ResponseWriter, req *http.Request, err error) {
    	log.Printf("%s %s -> %s failed: %v", req.Method, req.URL.Path, name, err)
    	if callIndex >= 0 {
    		tracker.FinishCall(groupID, callIndex, OutcomeFromError(err))
    	}
    	http.Error(w, "upstream error", http.StatusBadGateway)
    },
  }
  proxy.ServeHTTP(w, r)
}


func rewriteCard(resp *http.Response, from, to string) error {
  body, err := io.ReadAll(resp.Body)
  if err != nil {
    return err
  }
  resp.Body.Close()
  body = bytes.ReplaceAll(body, []byte(from), []byte(to))
  resp.Body = io.NopCloser(bytes.NewReader(body))
  resp.ContentLength = int64(len(body))
  resp.Header.Set("Content-Length", strconv.Itoa(len(body)))
  return nil
}

func debugHandler(w http.ResponseWriter, r *http.Request) {
  w.Header().Set("Content-Type", "application/json")
  json.NewEncoder(w).Encode(tracker.Snapshot())
}


func main() {
  startCleanup(time.Minute, time.Hour)

  mux := http.NewServeMux()
  mux.HandleFunc("/debug/requests", debugHandler)
  mux.HandleFunc("/", handler)
  log.Fatal(http.ListenAndServe(":8080", mux))
}
