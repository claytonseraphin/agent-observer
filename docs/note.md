### URL in go when parsed with  *url.URL from net/url
``Scheme: "http"
Host:   "server1:8080"
Path:   "/api"``

## Reverse Proxy routing in place:
``localhost:9000/news/       -> localhost:8001
localhost:9000/financial/  -> localhost:8002``


The Orchestrator in A2a makes to separate HTTP requests to an agent,
and the proxy has to handle bth differently

### Request 1: Fetch the agent card
`GET {base_url}/.well-known/agent-card.json`
The orchestrator's A2ACardResolver does this once, using the base URL you give it.
 When you point that URL at the proxy, the card request reaches the proxy,
 and the proxy forwards it to the real agent.


 ## Request 2: Send the message
 ``POST {url from the card}``

 ``Content-Type: application/json``


##
