# libre-graph-adapter-kit

Go building blocks for services that adapt OpenCloud's [LibreGraph API](https://github.com/opencloud-eu/libre-graph-api) to other protocols, such as [opencloud-music](https://github.com/opencloud-eu/opencloud-music) (Subsonic) and [immichframe-opencloud](https://github.com/opencloud-eu/immichframe-opencloud) (ImmichFrame).

Such adapters share a common shape: they accept requests in a foreign protocol, forward the caller's OpenCloud credentials (or a configured service account) to the Graph API, and stream file content back. This kit extracts the plumbing they all need, on top of the generated [libre-graph-api-go](https://github.com/opencloud-eu/libre-graph-api-go) client.

## Packages

- `auth` - the OpenCloud credential model (OIDC bearer token, or username + app token), request extraction, context plumbing, and helpers to attach credentials to outgoing calls (both `*http.Request` and libregraph contexts). Supports per-request credential forwarding (proxy-style adapters) as well as static service accounts.
- `graph` - convenience layer over the generated client: client construction (base URL, dev TLS), drive/space resolution by name, recursive children walks, KQL search and aggregation helpers, and signed download URL retrieval (`@microsoft.graph.downloadUrl`).
- `proxy` - stream a driveItem's content to an `http.ResponseWriter` with Range and conditional header passthrough, so media clients can seek and cache. A whitelist-based configuration of `httputil.ReverseProxy`: only download-relevant headers cross in either direction.

## Status

Early extraction from the two services named above; APIs are not stable yet.

## License

Apache-2.0
