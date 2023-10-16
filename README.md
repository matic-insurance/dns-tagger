# dns-tagger
Go library that allow to tag (claim ownership) of External DNS records in current k8s cluster. 

The motivation to create this library is:
- streamline cluster migrations - when creating new k8s cluster and migrating all your workloads that are managed by ExternalDNS
- cluster failover operations - when you want to switch all traffic to another cluster in failover scenarious  

## Description
### How it works

1. Connect to current k8s cluster and collect active endpoints (sources) managed by ExternalDNS in the cluster
2. Connect to DNS and collect all hosts and ExternalDNS TXT registry records responsible for those hosts
3. Modify ExternalDNS TXT registry records - so current ExternalDNS instance is able to manage DNS records that 
   are live in current cluster

### External DNS Setup
External DNS using DNS hosts (A, CNAME, AAAA) and registry records to make sure it updates only those records that it owns.
Thus allowing multiple instances of External DNS in same or different clusters to cooperate

Our External DNS is configured in the following way:
- Same TXTPrefix across different instances - so external share single registry and know what instance managing what record
- Different `TXTOwnerId` per cluster (e.g cluster name) - so same service running in multiple clusters do not fight over DNS records  

As result of such setup, whenever you manually or with dns-tag lib update registry record to another cluster - 
this cluster now owns the record and will update DNS to IP addresses in the cluster

### When dns-tag will claim ownership of specific record

Before updating registry following conditions should be true

1. Source (service, ingress, etc) with the DNS record (host, annotation) should be running in current cluster
2. DNS Zone for the DNS record allowed by input params
3. Registry record available for DNS record
4. Current owner for registry record is allowed by input params
5. Current owner is not up to date
6. Dry-Run not enabled

Due to misconfiguration or manual changes - it is possible that single DNS record will have multiple registry records. 
dns-tagger updates all registry records that it finds. 

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

TBD
## Description
// TODO(user): An in-depth paragraph about your project and overview of use

## Getting Started
You’ll need a Kubernetes cluster to run against. You can use [KIND](https://sigs.k8s.io/kind) to get a local cluster for testing, or run against a remote cluster.
**Note:** Your controller will automatically use the current context in your kubeconfig file (i.e. whatever cluster `kubectl cluster-info` shows).

### Running on the cluster
1. Install Instances of Custom Resources:

```sh
kubectl apply -f config/samples/
```

2. Build and push your image to the location specified by `IMG`:

```sh
make docker-build docker-push IMG=<some-registry>/dns-tagger:tag
```

3. Deploy the controller to the cluster with the image specified by `IMG`:

```sh
make deploy IMG=<some-registry>/dns-tagger:tag
```

### Uninstall CRDs
To delete the CRDs from the cluster:

```sh
make uninstall
```

### Undeploy controller
UnDeploy the controller from the cluster:

```sh
make undeploy
```

## Contributing
// TODO(user): Add detailed information on how you would like others to contribute to this project

### How it works
This project aims to follow the Kubernetes [Operator pattern](https://kubernetes.io/docs/concepts/extend-kubernetes/operator/).

It uses [Controllers](https://kubernetes.io/docs/concepts/architecture/controller/),
which provide a reconcile function responsible for synchronizing resources until the desired state is reached on the cluster.

### Test It Out
1. Install the CRDs into the cluster:

```sh
make install
```

2. Run your controller (this will run in the foreground, so switch to a new terminal if you want to leave it running):

```sh
make run
```

**NOTE:** You can also run this in one step by running: `make install run`

### Modifying the API definitions
If you are editing the API definitions, generate the manifests such as CRs or CRDs using:

```sh
make manifests
```

**NOTE:** Run `make --help` for more information on all potential `make` targets

More information can be found via the [Kubebuilder Documentation](https://book.kubebuilder.io/introduction.html)

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

