package http

import (
	"fmt"
	"net"
	"strings"
)

type Host struct {
	Name      string
	Namespace string
	Type      string
	Domain    string
	Port      string
	K8s       bool
}

type Parser struct {
	ClusterDomain string
}

func (p *Parser) ParseHost(h string, https bool) (*Host, error) {
	if h == "" {
		return nil, fmt.Errorf("host is empty")
	}
	host, port, err := net.SplitHostPort(h)
	if err != nil {
		if !strings.Contains(err.Error(), "missing port") {
			return nil, err
		}
		host = h
	}
	if port == "" && https {
		port = "443"
	}
	if port == "" {
		port = "80"
	}
	if !strings.HasSuffix(host, p.ClusterDomain) {
		return &Host{
			Domain: host,
			Port:   port,
			K8s:    false,
		}, nil
	}

	clusterDomain := p.ClusterDomain
	host = strings.TrimSuffix(host, fmt.Sprintf(".%s", clusterDomain))
	i := strings.LastIndex(host, ".")
	t := host[i+1:]
	host = host[:i]
	i = strings.LastIndex(host, ".")
	ns := host[i+1:]
	name := host[:i]
	return &Host{
		Name:      name,
		Namespace: ns,
		Type:      t,
		Domain:    clusterDomain,
		Port:      port,
		K8s:       true,
	}, nil
}
