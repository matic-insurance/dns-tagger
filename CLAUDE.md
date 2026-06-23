# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## What this tool does

`dns-tagger` is a Go CLI binary that transfers ownership of ExternalDNS TXT registry records between Kubernetes clusters. It is used for cluster migrations, failovers, and ingress-to-istio migrations.

ExternalDNS uses TXT records (`heritage=external-dns,external-dns/owner=<id>,external-dns/resource=<resource>`) to track which cluster instance owns each DNS host record. This tool rewrites those TXT records so a new cluster takes over management.

## Commands

```bash
make build        # compile to bin/dns-tagger
make test         # fmt + vet + go test ./... -coverprofile cover.out
make fmt          # go fmt ./...
make vet          # go vet ./...

# Run a single test
go test ./pkg/... -run TestSelector_UpdateRegistryRecords_SameOwner -v
go test ./registry/... -run TestRecord -v

# Run the binary (dry-run by default; add --apply to actually write changes)
./bin/dns-tagger \
  --source=istio-virtualservice \
  --previous-owner-id=OLD_CLUSTER \
  --current-owner-id=NEW_CLUSTER \
  --dns-zone=example.com
```

**Required environment variable (per provider used):** `DNSIMPLE_OAUTH` (OAuth token for DNSimple API) and/or `CLOUDFLARE_API_TOKEN` (API token for Cloudflare API). Only the provider(s) actually resolved for the given `--dns-zone` set need their token.

All CLI flags map to env vars automatically via kingpin `DefaultEnvars()` — e.g. `--current-owner-id` → `CURRENT_OWNER_ID`.

## Architecture

The data flow is linear:

```
K8s cluster → source → []Endpoint
DNSimple    → provider → []Zone (each with []Host, each Host with []Record)
                ↓
           pkg.Selector matches Endpoints to Hosts/Records and calls provider.UpdateRegistryRecord
```

### Packages

**`registry/`** — domain model, no external dependencies
- `Endpoint`: a host+resource pair sourced from K8s (e.g. `webserver.example.com` + `ingress/default/web`)
- `Host`: a DNS A/CNAME/AAAA record in a zone, with attached `RegistryRecords`
- `Record`: a parsed ExternalDNS TXT record — holds `Name`, `Owner`, `Resource`
- `Zone`: groups Hosts by DNS zone; `IsManagingEndpoint` checks suffix match
- `record.go` `IsManaging()`: the record name matching logic — only prefix on the first label is allowed, suffix on the first label is **not** (see README §Registry records matching)

**`source/`** — reads K8s resources and emits `[]Endpoint`
- `Source` interface: `Endpoints(ctx) ([]*registry.Endpoint, error)`
- `store.go`: `SingletonClientGenerator` for Kube/Istio clients; `ByNames`/`BuildWithConfig` factory
- `ingress.go`: reads `networking.k8s.io/v1` Ingress objects; uses `external-dns.alpha.kubernetes.io/hostname` annotation
- `istio_virtualservice.go`: reads Istio VirtualService + Gateway objects; same hostname annotation
- Label filtering (`--label` flag) uses OR semantics across all provided `key:value` or `key=value` selectors (`matchesAnyLabel` in `source.go`)

**`provider/`** — reads and writes DNS
- `Provider` interface: `ReadZones`, `UpdateRegistryRecord`, `Whoami`
- `dnsimple/dnsimple.go`: DNSimple implementation. `ReadZones` paginates all records, separates host records from TXT registry records, then links registry records to their hosts. `UpdateRegistryRecord` is a no-op in dry-run mode.
- `cloudflare/cloudflare.go`: Cloudflare implementation (auth via `CLOUDFLARE_API_TOKEN`). Same `ReadZones`/`UpdateRegistryRecord` shape; uses `cloudflare-go`. Cloudflare returns record names as FQDNs (no `name.zone` concatenation). `ListDNSRecords` auto-paginates when Page/PerPage are unset.
- `detect.go`: `DetectProvider` resolves a zone's NS records and `classifyNameservers` maps them to a provider kind (`ns.cloudflare.com` → cloudflare, `dnsimple.com` → dnsimple). Unknown → error.
- `routing.go`: `RoutingProvider` dispatches `UpdateRegistryRecord` to the provider owning each zone, so one run can span multiple providers.

**`pkg/`** — orchestration and configuration
- `config.go`: `Config` struct + kingpin flag parsing. Default mode is `owner`, default `--txt-prefix` is `edns-`, `--apply` defaults to false (dry-run)
- `selector.go`: `Selector` drives the two modes:
  - **owner mode** (`ClaimEndpointsOwnership`): updates TXT records where owner ∈ `--previous-owner-id` and owner ≠ `--current-owner-id`
  - **resource mode** (`ClaimEndpointsResource`): updates TXT records where the `resource` field differs from the K8s source resource (owner is preserved)
- `config.go` `--provider` flag: `auto` (default, detect per zone via NS records), `cloudflare`, or `dnsimple`

**`main.go`** — wires the above. `buildProvider` resolves each `--dns-zone` to a provider (auto-detected or forced via `--provider`), groups zones by provider, and returns a `RoutingProvider` when zones span more than one. This is the only place that touches `dnsimple.NewDnsimpleProvider` / `cloudflare.NewCloudflareProvider` directly.

## Key behaviours to be aware of

- Without `--apply`, all updates are logged but never written (dry-run). Always verify with dry-run before adding `--apply`.
- Multiple TXT records can match a single host (misconfiguration or manual edits); the selector updates all of them.
- Record-to-host matching (`Record.IsManaging`): TXT record name must share the same base domain as the host, and the first label may optionally be prefixed by `registry.Prefix` (`--txt-prefix`). A suffix on the first label is not accepted.
- DNSimple account ID is auto-detected via `Whoami` API call unless `--account-id` is set.
