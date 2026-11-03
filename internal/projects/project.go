package projects

import (
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
	URL       string
	CreatedAt time.Time
}
