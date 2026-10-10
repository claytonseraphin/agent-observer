package main

import(
  "bytes"
  "io"
  "net/http"
  "net/http/httputil"
  "net/url"
  "slices"
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

type RequestRecord struct {
  Agents     []string     `json:"agents"`
}

type Tracker struct {
  mu          sync.Mutex
  records     map[string]*RequestRecord
}


func (r *RequestRecord) Count() int {
  return len(r.Agents)
}


func NewTracker() *Tracker {
  return &Tracker{records: map[string]*RequestRecord{}}
}

func (t *Tracker) Record(key, agent string) (int, []string) {
	t.mu.Lock()
	defer t.mu.Unlock()

	rec, ok := t.records[key]
	if !ok {
		rec = &RequestRecord{}
		t.records[key] = rec
	}
	if !slices.Contains(rec.Agents, agent) {
		rec.Agents = append(rec.Agents, agent)
	}
	return len(rec.Agents), slices.Clone(rec.Agents)
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

  if r.Method == http.MethodPost {
    origin := "client"
  	key := r.Header.Get("X-Trace-Id")
  	if key == "" {
      origin = "inferred"
      address := clientAddress(r)
      key = assigner.Begin(address)
      defer assigner.End(address)
  	}
  	count, names := tracker.Record(key, name)
  	log.Printf("group=%s (%s) agents=%d names=%v", key, origin, count, names)
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
        if strings.HasSuffix(r.URL.Path, "agent-card.json") &&
        	resp.StatusCode == http.StatusOK &&
        	strings.Contains(resp.Header.Get("Content-Type"), "json") {
        	return rewriteCard(resp, target.String()+"/", publicBase+"/"+name+"/")
        }
        return nil
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
	tracker.mu.Lock()
	defer tracker.mu.Unlock()
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(tracker.records)
}


func main() {
  mux := http.NewServeMux()
  mux.HandleFunc("/debug/requests", debugHandler)
  mux.HandleFunc("/", handler)
  log.Fatal(http.ListenAndServe(":8080", mux))
}
// type ReverseProxy struct {
//   backends      []*url.URL
//   current       uint64
// }
//
//
// func NewReverseProxy(backendURLs []string) *ReverseProxy {
//   urls := make([]*url.URL, len(backendURLs))
//
//   for i, u := range backendURLs {
//     parsedURL, err := url.Parse(u)
//     if err != nil {
//       log.Fatalf("Invalid URL: %v", err)
//     }
//
//     urls[i] = parsedURL
//   }
//
//   return &ReverseProxy{backends: urls}
// }
//
//
// func (p *ReverseProxy) ServeHTTP(w http.ResponseWriter, r *http.Request) {
//   var index int
//
//   switch {
//   case strings.HasPrefix(r.URL.Host, "localhost:8001"):
//     index = 0
//     atomic.AddUint64(&p.current, 1)
//
//   case strings.HasPrefix(r.URL.Host, "localhost:8002"):
//     index = 1
//     atomic.AddUint64(&p.current, 1)
//
//   default:
//     return
//   }
//
//   proxy := httputil.NewSingleHostReverseProxy(p.backends[index])
//   proxy.ServeHTTP(w, r)
// }
//
//
// func main() {
//   backends := []string{"http://localhost:8001/", "http://localhost:8002/"}
//   proxy := NewReverseProxy(backends)
//   log.Fatal(http.ListenAndServe(":8080", proxy))
// }
