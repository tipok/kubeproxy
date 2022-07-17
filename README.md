# Kubeproxy

## Usage

Kubeproxy is forwarding all the traffic accessing k8s host names to the corresponding pods e.g.

http://service.namespace.svc.cluster.local:8080
http://service.namespace.svc.cluster.local:http
http://service.namespace.pod.cluster.local:8081

Two different types are supported: pod and svc. Named ports are supported.
All hostnames have to go to the domain `cluster.local` even though the domain differs from your actual k8s domain.

