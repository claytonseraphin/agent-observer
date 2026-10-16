# Why only POSTs are tracked, and why the group assigner exists

## Why only POSTs are tracked

The two kinds of request do different jobs:

| Request | What it is | Counted as an agent call? |
|---|---|---|
| `GET …/agent-card.json` | Discovery: the orchestrator asking what the agent is and where to send messages | No |
| `POST …/news/` | The actual work: a message sent to the agent | Yes |

The agent count means "how many agents did this question send work to." Only POSTs answer that, so the tracking block checks for `http.MethodPost`.

A GET could carry a trace ID header too, and nothing in the protocol forbids it. The proxy just doesn't treat it as an agent call. Card fetches could be recorded in the group as a separate kind of event if the UI should show discovery.

## Why the group assigner exists

It has nothing to do with GET versus POST. It exists because most orchestrators won't send a trace ID on any request. Without one, the proxy sees separate POSTs with nothing connecting them.

The assigner is the fallback: when no header comes in, it makes a group by guessing that calls from the same client address, close together in time, belong to one question.

The chain of reasoning:

1. A group needs an ID shared by every call of one question.
2. The best source is a trace ID header from the orchestrator, which works on any request.
3. Most orchestrators don't send one, so the proxy needs its own way to decide.
4. The group assigner is that way, and it only runs when step 2 produced nothing.
