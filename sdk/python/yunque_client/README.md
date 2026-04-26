# yunque_client — Yunque Agent Python SDK

Auto-generated Python client for the Yunque (云雀) Agent HTTP API.

- Source spec: [`docs/openapi.yaml`](../../../docs/openapi.yaml)
- Generator: [`openapi-python-client`](https://github.com/openapi-generators/openapi-python-client)
- 343 endpoints across 83 tag groups (chat, cognis, tasks, memory, knowledge, …)

> Note: this is the **API client** SDK. For writing Yunque plugins, see the
> sibling `yunque/` package in `sdk/python/`.

## Install

```bash
# From the repo root, after generating the spec:
cd sdk/python
pip install httpx attrs python-dateutil
```

(A `setup.py` will be added once we publish to PyPI; for now use it from the
local checkout.)

## Quick start

```python
from yunque_client import AuthenticatedClient
from yunque_client.api.cognis import list_cognis, generate_cogni

client = AuthenticatedClient(base_url="http://localhost:9090", token="<your-jwt>")

# List every Cogni
cognis = list_cognis.sync(client=client)
print(cognis)

# Self-generate a new Cogni from natural language
from yunque_client.models.generate_cogni_body import GenerateCogniBody  # adjust to actual model
result = generate_cogni.sync(client=client, body=GenerateCogniBody.from_dict({
    "description": "Build a code-review cogni for Go projects",
}))
print(result)
```

Async variants are available on every endpoint:

```python
result = await list_cognis.asyncio(client=client)
```

## Regenerating the SDK

After any route change, regenerate from the source repo root:

```bash
# 1. Refresh the OpenAPI spec
make openapi
# (or directly: go run ./cmd/openapi-gen)

# 2. Regenerate the Python client
cd sdk/python
python -m openapi_python_client generate \
    --path ../../docs/openapi.yaml \
    --config openapi-config.yaml \
    --meta none --overwrite
```

## Smoke test

```python
from yunque_client import Client, AuthenticatedClient
from yunque_client.api.cognis import list_cognis, generate_cogni
print("yunque_client imports OK")
```

## Status

- 509 endpoint stubs generated, 100+ data models
- Request/response schemas are mostly `Any` placeholders for now — the
  underlying spec is path-only. To enrich a specific endpoint:
  1. Hand-edit `docs/openapi.yaml` request/response bodies
  2. Regenerate the SDK
- Streaming endpoints (`/v1/chat/stream`, `/v1/events/stream`) are stubbed but
  will need manual SSE handling — `openapi-python-client` does not generate SSE
  consumers yet.

## Caveats

- Method inference is heuristic: a few endpoints expose both GET and POST
  stubs even when the handler accepts only one. Defer to the actual handler
  when in doubt.
- `operationId` follows two patterns:
  - Hand-curated cognis endpoints: `list_cognis`, `evolve_cogni`, etc.
  - Auto-generated: `<method>_<sanitised_path>` (e.g. `get_v1_chat`).
