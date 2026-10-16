package main

import "time"

type cleaner interface {
    Cleanup(maxAge time.Duration)
}

func startCleanup(every, maxAge time.Duration, cleaners ...cleaner) {
    go func() {
        ticker := time.NewTicker(every)
        defer ticker.Stop()
        for range ticker.C {
            for _, c := range cleaners {
                c.Cleanup(maxAge)
            }
        }
    }()
}
