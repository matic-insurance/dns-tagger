# dns-tagger
Go library that allow to tag (claim ownership) of External DNS records in current k8s cluster.

The motivation to create this library is:
- streamline cluster migrations - when creating new k8s cluster and migrating all your workloads that are managed by ExternalDNS
- cluster failover operations - when you want to switch all traffic to another cluster in failover scenarious
- application migration from ingress-nginx to istio and vice versa (resource mode)

## Description
### How it works

1. Connect to current k8s cluster and collect active endpoints (sources) managed by ExternalDNS in the cluster
2. Connect to DNS and collect all hosts and ExternalDNS TXT registry records responsible for those hosts
3. Modify ExternalDNS TXT registry records - so current ExternalDNS instance is able to manage DNS records that
   are live in current cluster (owner mode)
4. Modify ExternalDNS TXT registry records - so current DNS records will be pointed to specified resource (resource mode)

### External DNS Setup
External DNS using DNS hosts (A, CNAME, AAAA) and registry records to make sure it updates only those records that it owns.
Thus allowing multiple instances of External DNS in same or different clusters to cooperate

Our External DNS is configured in the following way:
- Same TXTPrefix across different instances - so external share single registry and know what instance managing what record
- Different `TXTOwnerId` per cluster (e.g cluster name) - so same service running in multiple clusters do not fight over DNS records

As result of such setup, whenever you manually or with dns-tag lib update registry record to another cluster -
this cluster now owns the record and will update DNS to IP addresses in the cluster

### When dns-tag will claim ownership of specific record (owner mode)

Before updating registry following conditions should be true

1. Source (service, ingress, etc) with the DNS record (host, annotation) should be running in current cluster
2. DNS Zone for the DNS record allowed by input params
3. Registry record available for DNS record
4. Current owner for registry record is allowed by input params
5. Current owner is not up to date
6. Dry-Run not enabled

Due to misconfiguration or manual changes - it is possible that single DNS record will have multiple registry records.
dns-tagger updates all registry records that it finds.

### When dns-tag will claim resource of specific record (resource mode)

Before updating registry following conditions should be true

1. Source (service, ingress, etc) with the DNS record (host, annotation) should be running in current cluster
2. DNS Zone for the DNS record allowed by input params
3. Registry record available for DNS record
4. Current owner for registry record is allowed by input params
5. Current resource does not match desired source
6. Dry-Run not enabled

### Registry records matching

Currently we match registry records by comparing name of TXT record and source DNS record, and checking contents
of TXT record to match External DNS registry format

If source has record `webserver.example.com`:
- matched `webserver.example.com`
- matched `ANYPREFIXwebserver.example.com`
- matched `webserverANYSUFFIX.example.com`
- not matched `webserver.another.com`
- not matched `PREFIXwebserverSUFFIX.example.com`
- not matched `4thlevel.webserver.example.com`

Content of TXT record should start from: `heritage=external-dns`

## Current State

This is an early prototype that Matic team is testing. At the moment we use it as standalone binary that
has limited support of infrastructure components, and external DNS configurations

Supported Sources:
  - Ingress source (no filtering)
  - Istio Virtual Service
  - Istio Gateway

Supported DNS providers
  - DNSimple

Supported External DNS Configs
  - Registry TXT
  - TXTOwnerId
  - TXTPrefix, TXTSuffix

If you find this tool usable in your environment - we are committed to provide some level of development,
accept new contributions, and/or transfer ownership to community.

## How to use

### Local run

**Note:** `dns-tagger` will automatically use the current context in your kubeconfig file (i.e. whatever cluster `kubectl cluster-info` shows).

1. Compile binary

   `make build`

2. Verify changes that will be made

   `./bin/dns-tagger --source=istio-virtualservice --previous-owner-id=PREVIUS_CLUSTER --current-owner-id=CURRENT_CLASTER --dns-zone=exmaple.com`

3. Apply changes (same command with `--apply` parameter)

   `./bin/dns-tagger --source=istio-virtualservice --previous-owner-id=PREVIUS_CLUSTER --current-owner-id=CURRENT_CLASTER --dns-zone=exmaple.com --apply`

### Compile binary

`make build`

### Run tests

`make test`


## Contributing
// TODO(user): Add detailed information on how you would like others to contribute to this project

## License

Copyright 2023.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.

