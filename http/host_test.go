package http

import "testing"

func TestParseHost(t *testing.T) {
	p := &Parser{ClusterDomain: "cluster.local"}
	t.Run("k8s with port", testParseHostK8sWithIntPort(p))
	t.Run("k8s with named port", testParseHostK8sWithNamedPort(p))
	t.Run("k8s without port https", testParseHostK8sWithWithoutPortHttps(p))
	t.Run("k8s without port non https", testParseHostK8sWithWithoutPortNonHttps(p))
	t.Run("non k8s without port https", testParseHostWithoutPortNonHttps(p))
}

func testParseHostK8sWithIntPort(p *Parser) func(t *testing.T) {
	return func(t *testing.T) {
		h := "home-notifier-redis-master.home-notifier.svc.cluster.local:8080"

		host, err := p.ParseHost(h, true)
		if err != nil {
			t.Errorf("failed to parse host: %v", err)
		}
		if host.Name != "home-notifier-redis-master" {
			t.Errorf("unexpected name: %s", host.Name)
		}
		if host.Namespace != "home-notifier" {
			t.Errorf("unexpected namespace: %s", host.Namespace)
		}
		if host.Type != "svc" {
			t.Errorf("unexpected type: %s", host.Type)
		}
		if host.Domain != "cluster.local" {
			t.Errorf("unexpected domain: %s", host.Domain)
		}
		if host.Port != "8080" {
			t.Errorf("unexpected port: %s", host.Port)
		}
		if !host.K8s {
			t.Errorf("unexpected k8s: %v", host.K8s)
		}
	}
}

func testParseHostK8sWithNamedPort(p *Parser) func(t *testing.T) {
	return func(t *testing.T) {
		h := "home-notifier-redis-master.home-notifier.pod.cluster.local:tcp-redis"

		host, err := p.ParseHost(h, true)
		if err != nil {
			t.Errorf("failed to parse host: %v", err)
		}
		if host.Name != "home-notifier-redis-master" {
			t.Errorf("unexpected name: %s", host.Name)
		}
		if host.Namespace != "home-notifier" {
			t.Errorf("unexpected namespace: %s", host.Namespace)
		}
		if host.Type != "pod" {
			t.Errorf("unexpected type: %s", host.Type)
		}
		if host.Domain != "cluster.local" {
			t.Errorf("unexpected domain: %s", host.Domain)
		}
		if host.Port != "tcp-redis" {
			t.Errorf("unexpected port: %s", host.Port)
		}
		if !host.K8s {
			t.Errorf("unexpected k8s: %v", host.K8s)
		}
	}
}

func testParseHostK8sWithWithoutPortHttps(p *Parser) func(t *testing.T) {
	return func(t *testing.T) {
		h := "home-notifier-redis-master.home-notifier.svc.cluster.local"

		host, err := p.ParseHost(h, true)
		if err != nil {
			t.Errorf("failed to parse host: %v", err)
		}
		if host.Name != "home-notifier-redis-master" {
			t.Errorf("unexpected name: %s", host.Name)
		}
		if host.Namespace != "home-notifier" {
			t.Errorf("unexpected namespace: %s", host.Namespace)
		}
		if host.Type != "svc" {
			t.Errorf("unexpected type: %s", host.Type)
		}
		if host.Domain != "cluster.local" {
			t.Errorf("unexpected domain: %s", host.Domain)
		}
		if host.Port != "443" {
			t.Errorf("unexpected port: %s", host.Port)
		}
		if !host.K8s {
			t.Errorf("unexpected k8s: %v", host.K8s)
		}
	}
}

func testParseHostK8sWithWithoutPortNonHttps(p *Parser) func(t *testing.T) {
	return func(t *testing.T) {
		h := "home-notifier-redis-master.home-notifier.svc.cluster.local"

		host, err := p.ParseHost(h, false)
		if err != nil {
			t.Errorf("failed to parse host: %v", err)
		}
		if host.Name != "home-notifier-redis-master" {
			t.Errorf("unexpected name: %s", host.Name)
		}
		if host.Namespace != "home-notifier" {
			t.Errorf("unexpected namespace: %s", host.Namespace)
		}
		if host.Type != "svc" {
			t.Errorf("unexpected type: %s", host.Type)
		}
		if host.Domain != "cluster.local" {
			t.Errorf("unexpected domain: %s", host.Domain)
		}
		if host.Port != "80" {
			t.Errorf("unexpected port: %s", host.Port)
		}
		if !host.K8s {
			t.Errorf("unexpected k8s: %v", host.K8s)
		}
	}
}

func testParseHostWithoutPortNonHttps(p *Parser) func(t *testing.T) {
	return func(t *testing.T) {
		h := "home-notifier-redis-master.home-notifier.svc.google.com"

		host, err := p.ParseHost(h, false)
		if err != nil {
			t.Errorf("failed to parse host: %v", err)
		}
		if host.K8s {
			t.Errorf("unexpected k8s: %v", host.K8s)
		}
		if host.Name != "" {
			t.Errorf("unexpected name: %s", host.Name)
		}
		if host.Namespace != "" {
			t.Errorf("unexpected namespace: %s", host.Namespace)
		}
		if host.Type != "" {
			t.Errorf("unexpected type: %s", host.Type)
		}
		if host.Domain != "home-notifier-redis-master.home-notifier.svc.google.com" {
			t.Errorf("unexpected domain: %s", host.Domain)
		}
		if host.Port != "80" {
			t.Errorf("unexpected port: %s", host.Port)
		}
	}
}
