package main

import (
  "net/url"
  "log"
)

func mustParse(s string) *url.URL {
  u, err := url.Parse(s)
  if err != nil {
    log.Fatal(err)
  }
  return u
}
