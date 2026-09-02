package k8s

import (
	"context"
	"fmt"
	"net/http"
	"os"

	"campfire/pkg/config"

	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/tools/portforward"
	"k8s.io/client-go/transport/spdy"
)

// Client encapsulates Kubernetes access for Campfire.
type Client struct {
	RestConfig *rest.Config
	Clientset  kubernetes.Interface
	Dynamic    dynamic.Interface
	Namespace  string
}

// NewClient creates a new Client based on the Campfire configuration.
func NewClient(cfg *config.Config) (*Client, error) {
	var restConfig *rest.Config
	var err error

	if cfg.Server != "" && cfg.Token != "" {
		// Use direct API token & Server configuration
		rc := &rest.Config{
			Host:        cfg.Server,
			BearerToken: cfg.Token,
			TLSClientConfig: rest.TLSClientConfig{
				Insecure: cfg.InsecureSkipTLSVerify,
			},
		}
		if cfg.CAData != "" {
			rc.TLSClientConfig.CAData = []byte(cfg.CAData)
		}
		restConfig = rc
	} else {
		// Fallback to kubeconfig
		loadingRules := clientcmd.NewDefaultClientConfigLoadingRules()
		if cfg.KubeconfigPath != "" {
			loadingRules.ExplicitPath = cfg.KubeconfigPath
		}
		clientConfig := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(loadingRules, &clientcmd.ConfigOverrides{})
		restConfig, err = clientConfig.ClientConfig()
		if err != nil {
			return nil, fmt.Errorf("failed to build kubernetes config: %w", err)
		}
	}

	cs, err := kubernetes.NewForConfig(restConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize kubernetes clientset: %w", err)
	}

	dyn, err := dynamic.NewForConfig(restConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize dynamic client: %w", err)
	}

	ns := cfg.Namespace
	if ns == "" {
		ns = "default"
	}

	return &Client{
		RestConfig: restConfig,
		Clientset:  cs,
		Dynamic:    dyn,
		Namespace:  ns,
	}, nil
}

// PortForward starts an SPDY port-forward to a pod and blocks until stopCh is closed.
func (c *Client) PortForward(ctx context.Context, podName string, localPort, podPort int, readyCh chan struct{}, stopCh chan struct{}) error {
	roundTripper, upgrader, err := spdy.RoundTripperFor(c.RestConfig)
	if err != nil {
		return fmt.Errorf("failed to create round tripper: %w", err)
	}

	url := c.Clientset.CoreV1().RESTClient().Post().
		Resource("pods").
		Namespace(c.Namespace).
		Name(podName).
		SubResource("portforward").URL()

	dialer := spdy.NewDialer(upgrader, &http.Client{Transport: roundTripper}, http.MethodPost, url)
	ports := []string{fmt.Sprintf("%d:%d", localPort, podPort)}

	pf, err := portforward.New(dialer, ports, stopCh, readyCh, nil, os.Stderr)
	if err != nil {
		return fmt.Errorf("failed to create port forward: %w", err)
	}

	return pf.ForwardPorts()
}
