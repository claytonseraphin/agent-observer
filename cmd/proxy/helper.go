package main

import (
  "net/url"
  "log"
  "os"
)

func mustParse(s string) *url.URL {
  u, err := url.Parse(s)
  if err != nil {
    log.Fatal(err)
  }
  return u
}

func envOr(key, fallback string) string{
  if v := os.Getenv(key); v != "" {
    return v
  }
  return fallback
}
