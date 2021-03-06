# Kubeproxy

A simple proxy routing all the traffic resolvable in k8s cluster to the cluster.
The proxy is supporting kube-dns like default domain and service discovery, without using kube-dns directly, but
mimicking the default behaviour, therefor the host name always has to have the following schema:

```
service.namespace.svc.cluster.local
```


## Install

### Simple

Download one of the binaries for a release and run it.

### macOS Homebrew

```shell
brew tap tipok/kubeproxy
brew install kubeproxy
```

## Usage

Kubeproxy is forwarding all the traffic accessing k8s host names to the corresponding pods e.g.

```
http://service.namespace.svc.cluster.local:8080
http://service.namespace.svc.cluster.local:http
http://service.namespace.pod.cluster.local:8081
```

Two different types are supported: pod and svc. Named ports are supported.
All hostnames have to end with the cluster name which can be configured with `--cluster-domain`.
