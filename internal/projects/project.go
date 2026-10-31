package projects

import (
	"net/url"
	"time"
)

type Project struct {
	ID        string
	Name      string
	KeyHash   string
	KeyHint   string
	CreatedAt time.Time
	Agents    []Agent
}

type Agent struct {
	Name      string
	URL       url.URL
	CreatedAt time.Time
}
