# PhotoKeep AI gateway

To run the gateway on a different machine without installing the desktop app
there, build a standalone package as described in
[Remote gateway deployment](remote-ai-gateway.md).

The desktop app can send images to one managed endpoint: `POST /v1/analyze`.
The gateway holds provider credentials; the desktop app does not
receive or store those provider keys. `GET /health` reports readiness. Search
continues to use the app's local SQLite index of saved captions and tags.

The desktop defaults to `https://vision.k1god.com/v1/analyze`; its provider is
selected on the gateway. For local development, set
`PHOTOKEEP_AI_GATEWAY_URL=http://127.0.0.1:8081/v1/analyze`; `./run.sh` starts
the local gateway when needed using `.env.gateway`. See
[xKiro gateway](xkiro-experiment.md) for setup.

## Run locally

Set `PHOTOKEEP_GATEWAY_PROVIDER` and provider keys in `.env.gateway` on the
**gateway server**. Run:

```sh
./scripts/run-ai-gateway.sh
```

The desktop's settings do not ask for provider keys. Once a public HTTPS URL is deployed, embed it in
desktop builds using
`-ldflags "-X main.managedAIEndpoint=https://your-host/v1/analyze"`.
`PHOTOKEEP_AI_GATEWAY_URL` can override the URL during development. The gateway
must be running for analysis; searching saved captions and tags remains local.

The gateway binds to loopback by default. Before public exposure, put it
behind HTTPS with request rate limits and access controls; otherwise anyone
could consume provider quotas. `PHOTOKEEP_GATEWAY_TOKEN` can protect a private
deployment, but do not embed that shared secret in public desktop binaries.

Provider keys are read at server startup. Change them in the server environment
and restart the gateway to rotate keys. No desktop reinstall is needed.
