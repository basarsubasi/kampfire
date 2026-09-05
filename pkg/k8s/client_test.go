package k8s

import (
	"net/url"
	"testing"

	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/portforward"
)

func TestNewExecutor(t *testing.T) {
	client := &Client{
		RestConfig: &rest.Config{
			Host: "https://127.0.0.1:6443",
		},
	}

	u, err := url.Parse("https://127.0.0.1:6443/api/v1/namespaces/default/pods/test/exec")
	if err != nil {
		t.Fatalf("failed to parse url: %v", err)
	}

	exec, err := client.NewExecutor("POST", u)
	if err != nil {
		t.Fatalf("expected NewExecutor to succeed, got error: %v", err)
	}
	if exec == nil {
		t.Fatalf("expected non-nil executor")
	}
}

func TestNewPortForwardDialer(t *testing.T) {
	client := &Client{
		RestConfig: &rest.Config{
			Host: "https://127.0.0.1:6443",
		},
	}

	u, err := url.Parse("https://127.0.0.1:6443/api/v1/namespaces/default/pods/test/portforward")
	if err != nil {
		t.Fatalf("failed to parse url: %v", err)
	}

	dialer, err := portforward.NewSPDYOverWebsocketDialer(u, client.RestConfig)
	if err != nil {
		t.Fatalf("expected NewSPDYOverWebsocketDialer to succeed, got error: %v", err)
	}
	if dialer == nil {
		t.Fatalf("expected non-nil dialer")
	}
}
