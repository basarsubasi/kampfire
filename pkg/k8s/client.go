package k8s

import (
	"context"
	"fmt"
	"io"
	"net/url"
	"os"
	"strings"

	"github.com/basarsubasi/kampfire/pkg/config"

	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/tools/portforward"
	"k8s.io/client-go/tools/remotecommand"
)

// Client encapsulates Kubernetes access for Campfire.
type Client struct {
	RestConfig *rest.Config
	Clientset  kubernetes.Interface
	Dynamic    dynamic.Interface
	Namespace  string
	Context    string
}

// NewClient creates a new Client based on the Kampfire configuration and optional CLI namespace override.
func NewClient(cfg *config.Config, namespaceOverride string) (*Client, error) {
	loadingRules := config.NewClientConfigLoadingRules(cfg)
	clientConfig := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(loadingRules, &clientcmd.ConfigOverrides{})
	restConfig, err := clientConfig.ClientConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to build kubernetes config: %w", err)
	}

	// Token override: check KAMPFIRE_API_TOKEN / CAMPFIRE_API_TOKEN env var or config token
	token := os.Getenv("KAMPFIRE_API_TOKEN")
	if token == "" && cfg != nil {
		token = cfg.Token
	}
	if token != "" {
		restConfig.BearerToken = strings.TrimSpace(token)
		// Clear client certificate auth from kubeconfig so bearer token takes precedence
		restConfig.CertData = nil
		restConfig.CertFile = ""
		restConfig.KeyData = nil
		restConfig.KeyFile = ""
	}

	cs, err := kubernetes.NewForConfig(restConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize kubernetes clientset: %w", err)
	}

	dyn, err := dynamic.NewForConfig(restConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize dynamic client: %w", err)
	}

	// Resolve namespace with proper precedence (CLI -n > active kubeconfig context > "default")
	ns := config.ResolveNamespace(namespaceOverride, cfg)

	var currentContext string
	if raw, err := clientConfig.RawConfig(); err == nil {
		currentContext = raw.CurrentContext
	}

	return &Client{
		RestConfig: restConfig,
		Clientset:  cs,
		Dynamic:    dyn,
		Namespace:  ns,
		Context:    currentContext,
	}, nil
}

// NewExecutor creates a WebSocket remotecommand executor for streaming container execution.
func (c *Client) NewExecutor(method string, u *url.URL) (remotecommand.Executor, error) {
	return remotecommand.NewWebSocketExecutor(c.RestConfig, method, u.String())
}

// PortForward starts a WebSocket port-forward to a pod on 0.0.0.0 and blocks until stopCh is closed.
func (c *Client) PortForward(ctx context.Context, podName string, localPort, podPort int, readyCh chan struct{}, stopCh chan struct{}) error {
	return c.PortForwardAddresses(ctx, podName, []string{"0.0.0.0"}, []string{fmt.Sprintf("%d:%d", localPort, podPort)}, readyCh, stopCh, nil, os.Stderr)
}

// PortForwardPorts starts a WebSocket port-forward to a pod with one or more port mappings.
func (c *Client) PortForwardPorts(ctx context.Context, podName string, ports []string, readyCh chan struct{}, stopCh chan struct{}, stdout, stderr io.Writer) error {
	return c.PortForwardAddresses(ctx, podName, []string{"127.0.0.1"}, ports, readyCh, stopCh, stdout, stderr)
}

// PortForwardAddresses starts a WebSocket port-forward to a pod with specific listen addresses and port mappings.
func (c *Client) PortForwardAddresses(ctx context.Context, podName string, addresses []string, ports []string, readyCh chan struct{}, stopCh chan struct{}, stdout, stderr io.Writer) error {
	url := c.Clientset.CoreV1().RESTClient().Post().
		Resource("pods").
		Namespace(c.Namespace).
		Name(podName).
		SubResource("portforward").URL()

	dialer, err := portforward.NewSPDYOverWebsocketDialer(url, c.RestConfig)
	if err != nil {
		return fmt.Errorf("failed to create port-forward dialer: %w", err)
	}

	if len(addresses) == 0 {
		addresses = []string{"0.0.0.0"}
	}

	pf, err := portforward.NewOnAddresses(dialer, addresses, ports, stopCh, readyCh, stdout, stderr)
	if err != nil {
		return fmt.Errorf("failed to create port forward: %w", err)
	}

	return pf.ForwardPorts()
}

