# PhotoKeep AI gateway source

This folder is the gateway source only. It has one configurable server entry
point, the shared API implementation, the PhotoKeep Go package it imports, and the
Go module files trimmed to this package's dependencies. It does not include
the desktop frontend, test files, benchmark tool, image database, photos, or
provider credentials.

## Run on another machine

Install Go 1.26.4 or newer, copy this folder to the machine, then:

```sh
cp gateway.env.example gateway.env
chmod 600 gateway.env
# Edit gateway.env, select a provider, and add the provider's keys.
go build ./services/ai-gateway/cmd/photokeep-ai-gateway
./start.sh
```

`start.sh` runs the binary `photokeep-ai-gateway` using the provider selected
by `PHOTOKEEP_GATEWAY_PROVIDER` on
`127.0.0.1:8081`. Choose `xkiro` or `cloudflare-thaillm` in `gateway.env`.
Run the server with a service manager if it should survive logout or reboot.
This server calls hosted AI APIs and does not download a model or require a GPU.

For xKiro, set `XKIRO_API_KEYS=key1,key2,...`. For Cloudflare/ThaiLLM, set
`CLOUDFLARE_ACCOUNTS=account1:token1,account2:token2,...` and
`THAILLM_API_KEYS=key1,key2,...`. The gateway round-robins image requests
across configured keys. Its default xKiro concurrency is 64;
`PHOTOKEEP_GATEWAY_CONCURRENCY` can override it from 1 to 128. When xKiro
returns an IP-scoped 429, the gateway stops retrying other keys and sends new
images to `https://vision.k1god.com/v1/analyze` until `Retry-After` expires,
then probes xKiro once. Set `PHOTOKEEP_GATEWAY_FALLBACK_URL=off` to disable
this fallback. Keys sharing one provider quota still share that quota.

The gateway exposes `GET /health` and `POST /v1/analyze` with one multipart
`file` field. To connect PhotoKeep over an SSH tunnel or private HTTPS, follow
the connection instructions in the source repository's
`docs/remote-ai-gateway.md`. For the SSH example, run
`ssh -N -L 18081:127.0.0.1:8081 user@GATEWAY_HOST` on the PhotoKeep machine,
then launch the app with
`PHOTOKEEP_AI_GATEWAY_URL=http://127.0.0.1:18081/v1/analyze ./run.sh`.
The app keeps its image database and search index locally.

Keep the HTTP server bound to loopback unless private network access and
authentication are configured. The desktop does not send a gateway secret,
and an exposed endpoint can spend provider quota.
