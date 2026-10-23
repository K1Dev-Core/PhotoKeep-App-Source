# PhotoKeep xKiro gateway

The local development AI gateway listens on `127.0.0.1:8081` and implements `GET /health` and `POST /v1/analyze` (multipart `file`). Choose xKiro or Cloudflare/ThaiLLM in `.env.gateway`; provider keys stay on the gateway server.

## Start

Set `PHOTOKEEP_GATEWAY_PROVIDER=xkiro` and `XKIRO_API_KEYS` in `.env.gateway`, then run `./scripts/run-ai-gateway.sh`. The default model is `mistralai/mistral-medium-3.5`. Keep this service on loopback unless access controls are configured. `/health` checks configured provider readiness.

The service reduces each uploaded image to a JPEG no larger than 512 pixels before sending it to the xKiro OpenAI-compatible chat API. It requests a Thai caption and dynamic Thai tags, removes non-Thai or duplicate tags, and rejects captions without Thai text. A successful response uses the existing PhotoKeep analysis schema.

Requests are paced at 600 ms apart by default. `XKIRO_MIN_INTERVAL_MS` can adjust this on the gateway; a 429 pauses sending briefly and increases spacing. The desktop queue stops feeding new images on 429 and resumes pending work after the provider's delay.
