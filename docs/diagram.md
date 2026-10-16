WRITE SIDE (per request)
┌──────────────┐   POST/GET    ┌────────────────────────────────┐   forwards   ┌────────────┐
│ Orchestrator │ ────────────▶ │  handler  (route by /news/..)  │ ───────────▶ │ news :8001 │
│  (Python)    │ ◀──────────── │                                │ ◀─────────── │ financial  │
└──────────────┘   response    │  1. assigner.Begin → groupID   │   response   │   :8002    │
        │  2. tracker.StartCall          │              └────────────┘
        │  3. ModifyResponse/ErrorHandler│
        │     → tracker.FinishCall       │
        └───────────────┬────────────────┘
                        │ writes (under lock)
                        ▼
        ┌────────────────────────────────┐
        │ Tracker.groups                 │◀─── cleanup goroutine
        │ (GroupRecord → CallRecord)     │     (every minute, removes
        │ GroupAssigner.byClient         │      old groups/clients)
        └───────────────┬────────────────┘
                        │ Snapshot() copies into views
                        ▼
READ SIDE (new API)
┌──────────────┐   GET         ┌────────────────────────────────┐
│  Portal UI   │ ────────────▶ │ /api/groups          (list)    │
│ (polls every │ ◀──────────── │ /api/groups/{id}     (one)     │
│  2 seconds)  │   JSON        │ /api/health                    │
└──────────────┘               │  → CORS wrapper → GroupView[]  │
        └────────────────────────────────┘
