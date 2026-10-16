# agent-observer
An observability tool for AI agents and their orchestrators built on JSON-RPC-based protocols.

## Temporary command to run the proxy and the api locally

`docker build -t agent-observer .``

`
docker build -t agent-observer .
docker run --rm -p 8080:8080 \
  -e NEWS_URL=http://host.docker.internal:8001 \
  -e FINANCIAL_URL=http://host.docker.internal:8002 \
  agent-observer
`

# Shorter Approach with Makefile
`make dev` to build compose.dv
`make help` to see your different options
