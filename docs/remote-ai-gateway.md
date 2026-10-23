# PhotoKeep AI gateway on a separate machine

If you want the Go source instead of prebuilt binaries, run
`./scripts/package-ai-gateway-source.sh` and use the generated
`dist/photokeep-ai-gateway-source.tar.gz` archive. Its own README explains
how to run it on the destination machine.

This package contains a standalone Go binary. The gateway machine does not need
the PhotoKeep desktop app, Node.js, Wails, Go, or a local AI model. It does need
outbound HTTPS access to the configured AI provider.

## Build and transfer

From the PhotoKeep source tree, run
`./scripts/package-ai-gateway.sh linux/amd64` (or `linux/arm64`). Copy the
resulting `dist/photokeep-ai-gateway-linux-amd64.tar.gz` archive to the other
machine, extract it, and verify `sha256sum -c SHA256SUMS` there. The package does not
contain provider credentials.

On the gateway machine:

```sh
cd photokeep-ai-gateway-linux-amd64
cp gateway.env.example gateway.env
chmod 600 gateway.env
# Edit gateway.env, select a provider, and add its keys.
./start.sh
```

For a persistent Linux service, run `start.sh` under a dedicated unprivileged
user with a service manager such as systemd. Keep `gateway.env` readable only
by that user. The default listen address is loopback, so the HTTP endpoint is
not accessible from the public network.

## Connect the desktop securely

For a quick private setup, forward the port with SSH from the PhotoKeep machine:

```sh
ssh -N -L 18081:127.0.0.1:8081 user@GATEWAY_HOST
```

Then launch the desktop app from a second terminal:

```sh
PHOTOKEEP_AI_GATEWAY_URL=http://127.0.0.1:18081/v1/analyze ./run.sh
```

For an always-on deployment, publish the gateway through private HTTPS
(for example, an authenticated private network and HTTPS reverse proxy), then
set `PHOTOKEEP_AI_GATEWAY_URL=https://YOUR_PRIVATE_HOST/v1/analyze` when
launching the app. `GET /health` must be reachable at the same host. The
desktop currently does not send a gateway authentication token, so do not
expose `POST /v1/analyze` on an unauthenticated public URL: each request spends
provider quota. Restrict access at the network layer. Provider keys remain on
the gateway machine. Search and the photo database remain on the desktop.

The gateway exposes `GET /health` and `POST /v1/analyze` with a multipart
`file` field. The desktop sends each image separately through
its existing persistent job queue; this gateway package does not introduce a
batch API.
