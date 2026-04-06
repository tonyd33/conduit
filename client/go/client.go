package conduitclient

import (
	"context"
	"fmt"
	"time"

	"github.com/nats-io/nats.go"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

// ResponseHandler is a function that processes responses from an Exchange
type ResponseHandler func(*Message) error

// ExchangeClient is a client for interacting with Conduit Exchange workers
type ExchangeClient struct {
	nc            *nats.Conn
	js            nats.JetStreamContext
	subjectInput  string
	subjectOutput string

	// Public fields for Conduit to reference
	Name      string
	Namespace string
}

// Conduit manages Exchange lifecycle via Kubernetes API
type Conduit struct {
	k8sClient     kubernetes.Interface
	dynamicClient dynamic.Interface
	namespace     string
	natsURL       string
}

// EnvVar represents an environment variable
type EnvVar struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

// VolumeMount represents a volume mount configuration
type VolumeMount struct {
	Name      string `json:"name"`
	MountPath string `json:"mountPath"`
	ReadOnly  *bool  `json:"readOnly,omitempty"`
}

// Volume represents a volume that can be mounted by containers
type Volume struct {
	Name                  string                               `json:"name"`
	EmptyDir              map[string]interface{}               `json:"emptyDir,omitempty"`
	ConfigMap             *corev1.ConfigMapVolumeSource        `json:"configMap,omitempty"`
	Secret                *corev1.SecretVolumeSource           `json:"secret,omitempty"`
	PersistentVolumeClaim *corev1.PersistentVolumeClaimVolumeSource `json:"persistentVolumeClaim,omitempty"`
}

// ExchangeRequest represents a request to create an Exchange via Kubernetes API
type ExchangeRequest struct {
	Name         string        `json:"name"`
	Namespace    string        `json:"namespace,omitempty"`
	Image        string        `json:"image"`
	Env          []EnvVar      `json:"env,omitempty"`
	VolumeMounts []VolumeMount `json:"volumeMounts,omitempty"`
	Volumes      []Volume      `json:"volumes,omitempty"`
}

// ExchangeResource represents an Exchange from the API
type ExchangeResource struct {
	Name           string         `json:"name"`
	Namespace      string         `json:"namespace"`
	UID            string         `json:"uid"`
	Phase          string         `json:"phase"`
	Message        string         `json:"message,omitempty"`
	ConnectionInfo ConnectionInfo `json:"connectionInfo"`
}

// ConnectionInfo provides NATS connection details
type ConnectionInfo struct {
	NATSURL       string `json:"natsURL"`
	StreamName    string `json:"streamName"`
	InputSubject  string `json:"inputSubject"`
	OutputSubject string `json:"outputSubject"`
}

// NewConduit creates a new Conduit Kubernetes client
func NewConduit(natsURL, namespace string) (*Conduit, error) {
	// Load kubeconfig
	loadingRules := clientcmd.NewDefaultClientConfigLoadingRules()
	configOverrides := &clientcmd.ConfigOverrides{}
	kubeConfig := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(loadingRules, configOverrides)

	config, err := kubeConfig.ClientConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to load kubeconfig: %w", err)
	}

	// Create Kubernetes client
	k8sClient, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create Kubernetes client: %w", err)
	}

	// Create dynamic client for custom resources
	dynamicClient, err := dynamic.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create dynamic client: %w", err)
	}

	if namespace == "" {
		namespace = "default"
	}

	return &Conduit{
		k8sClient:     k8sClient,
		dynamicClient: dynamicClient,
		namespace:     namespace,
		natsURL:       natsURL,
	}, nil
}

// CreateExchangeClient creates a new Exchange via Kubernetes API and returns a connected client
func (c *Conduit) CreateExchangeClient(req ExchangeRequest) (*ExchangeClient, error) {
	// Create the Exchange
	err := c.createExchange(req)
	if err != nil {
		return nil, err
	}

	namespace := req.Namespace
	if namespace == "" {
		namespace = c.namespace
	}

	// Wait for it to be ready
	resource, err := c.waitForExchangeReady(req.Name, namespace, 60*time.Second)
	if err != nil {
		return nil, err
	}

	// Connect to NATS
	nc, err := nats.Connect(resource.ConnectionInfo.NATSURL,
		nats.MaxReconnects(-1),
		nats.ReconnectWait(2*time.Second),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to NATS: %w", err)
	}

	// Create JetStream context
	js, err := nc.JetStream()
	if err != nil {
		nc.Close()
		return nil, fmt.Errorf("failed to create JetStream context: %w", err)
	}

	return &ExchangeClient{
		nc:            nc,
		js:            js,
		subjectInput:  resource.ConnectionInfo.InputSubject,
		subjectOutput: resource.ConnectionInfo.OutputSubject,
		Name:          resource.Name,
		Namespace:     resource.Namespace,
	}, nil
}

// DeleteExchangeClient deletes an Exchange via Kubernetes API
func (c *Conduit) DeleteExchangeClient(exchange *ExchangeClient) error {
	return c.deleteExchange(exchange.Name, exchange.Namespace)
}

// Send sends a message to the Exchange without waiting for a response
func (c *ExchangeClient) Send(ctx context.Context, payload interface{}) error {
	msg, err := NewDataMessage(payload)
	if err != nil {
		return fmt.Errorf("failed to create message: %w", err)
	}

	data, err := MarshalMessage(msg)
	if err != nil {
		return fmt.Errorf("failed to marshal message: %w", err)
	}

	_, err = c.js.Publish(c.subjectInput, data)
	if err != nil {
		return fmt.Errorf("failed to publish message: %w", err)
	}

	return nil
}

// Request sends a message and waits for a single response with a timeout
// This uses a temporary inbox subscription for the response
func (c *ExchangeClient) Request(ctx context.Context, payload interface{}, timeout time.Duration) (*Message, error) {
	msg, err := NewDataMessage(payload)
	if err != nil {
		return nil, fmt.Errorf("failed to create message: %w", err)
	}

	data, err := MarshalMessage(msg)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal message: %w", err)
	}

	// Create a temporary subscription for the response
	// We'll correlate by message ID
	responseChan := make(chan *Message, 1)
	errChan := make(chan error, 1)

	// Subscribe to output subject
	sub, err := c.nc.Subscribe(c.subjectOutput, func(natsMsg *nats.Msg) {
		respMsg, err := UnmarshalMessage(natsMsg.Data)
		if err != nil {
			errChan <- err
			return
		}

		// Check if this response correlates to our request
		// Note: This is a simple implementation. For production, you might want
		// to add correlation IDs in metadata
		responseChan <- respMsg
	})
	if err != nil {
		return nil, fmt.Errorf("failed to subscribe to output: %w", err)
	}
	defer sub.Unsubscribe()

	// Publish the request
	_, err = c.js.Publish(c.subjectInput, data)
	if err != nil {
		return nil, fmt.Errorf("failed to publish message: %w", err)
	}

	// Wait for response or timeout
	select {
	case resp := <-responseChan:
		return resp, nil
	case err := <-errChan:
		return nil, err
	case <-time.After(timeout):
		return nil, fmt.Errorf("request timeout after %v", timeout)
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

// Subscribe subscribes to all responses from the Exchange
// The handler will be called for each response message
func (c *ExchangeClient) Subscribe(ctx context.Context, handler ResponseHandler) error {
	// Subscribe to output subject
	sub, err := c.nc.Subscribe(c.subjectOutput, func(natsMsg *nats.Msg) {
		msg, err := UnmarshalMessage(natsMsg.Data)
		if err != nil {
			// Log error but continue processing
			return
		}

		// Call handler
		_ = handler(msg)
	})
	if err != nil {
		return fmt.Errorf("failed to subscribe: %w", err)
	}

	// Wait for context cancellation
	<-ctx.Done()

	// Unsubscribe
	sub.Unsubscribe()

	return ctx.Err()
}

// SubscribeWithQueue subscribes to responses using a queue group
// This allows multiple clients to load-balance response processing
func (c *ExchangeClient) SubscribeWithQueue(ctx context.Context, queueGroup string, handler ResponseHandler) error {
	// Subscribe to output subject with queue group
	sub, err := c.nc.QueueSubscribe(c.subjectOutput, queueGroup, func(natsMsg *nats.Msg) {
		msg, err := UnmarshalMessage(natsMsg.Data)
		if err != nil {
			return
		}

		_ = handler(msg)
	})
	if err != nil {
		return fmt.Errorf("failed to subscribe: %w", err)
	}

	// Wait for context cancellation
	<-ctx.Done()

	// Unsubscribe
	sub.Unsubscribe()

	return ctx.Err()
}

// Close closes the NATS connection
func (c *ExchangeClient) Close() error {
	if c.nc != nil {
		c.nc.Close()
	}
	return nil
}

// Private Kubernetes API helper methods for Conduit

func (c *Conduit) createExchange(req ExchangeRequest) error {
	namespace := req.Namespace
	if namespace == "" {
		namespace = c.namespace
	}

	// Build the container spec
	container := map[string]interface{}{
		"name":  "exchange",
		"image": req.Image,
	}

	if len(req.Env) > 0 {
		envVars := make([]map[string]interface{}, len(req.Env))
		for i, e := range req.Env {
			envVars[i] = map[string]interface{}{
				"name":  e.Name,
				"value": e.Value,
			}
		}
		container["env"] = envVars
	}

	if len(req.VolumeMounts) > 0 {
		volumeMounts := make([]map[string]interface{}, len(req.VolumeMounts))
		for i, vm := range req.VolumeMounts {
			mount := map[string]interface{}{
				"name":      vm.Name,
				"mountPath": vm.MountPath,
			}
			if vm.ReadOnly != nil {
				mount["readOnly"] = *vm.ReadOnly
			}
			volumeMounts[i] = mount
		}
		container["volumeMounts"] = volumeMounts
	}

	// Build the pod spec
	podSpec := map[string]interface{}{
		"containers": []interface{}{container},
	}

	if len(req.Volumes) > 0 {
		volumes := make([]map[string]interface{}, len(req.Volumes))
		for i, vol := range req.Volumes {
			volume := map[string]interface{}{
				"name": vol.Name,
			}
			if vol.EmptyDir != nil {
				volume["emptyDir"] = vol.EmptyDir
			} else if vol.ConfigMap != nil {
				volume["configMap"] = vol.ConfigMap
			} else if vol.Secret != nil {
				volume["secret"] = vol.Secret
			} else if vol.PersistentVolumeClaim != nil {
				volume["persistentVolumeClaim"] = vol.PersistentVolumeClaim
			}
			volumes[i] = volume
		}
		podSpec["volumes"] = volumes
	}

	// Build the Exchange CR
	exchangeCR := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "conduit.mnke.org/v1alpha1",
			"kind":       "Exchange",
			"metadata": map[string]interface{}{
				"name":      req.Name,
				"namespace": namespace,
			},
			"spec": map[string]interface{}{
				"template": map[string]interface{}{
					"spec": podSpec,
				},
			},
		},
	}

	// Define the GVR for Exchange CRD
	gvr := schema.GroupVersionResource{
		Group:    "conduit.mnke.org",
		Version:  "v1alpha1",
		Resource: "exchanges",
	}

	// Create the Exchange CR
	_, err := c.dynamicClient.Resource(gvr).Namespace(namespace).Create(
		context.Background(),
		exchangeCR,
		metav1.CreateOptions{},
	)
	if err != nil {
		return fmt.Errorf("failed to create Exchange: %w", err)
	}

	return nil
}

func (c *Conduit) getExchange(name, namespace string) (*ExchangeResource, error) {
	// Define the GVR for Exchange CRD
	gvr := schema.GroupVersionResource{
		Group:    "conduit.mnke.org",
		Version:  "v1alpha1",
		Resource: "exchanges",
	}

	// Get the Exchange CR
	exchangeUnstructured, err := c.dynamicClient.Resource(gvr).Namespace(namespace).Get(
		context.Background(),
		name,
		metav1.GetOptions{},
	)
	if err != nil {
		return nil, fmt.Errorf("failed to get Exchange: %w", err)
	}

	// Extract fields from unstructured data
	exchange := exchangeUnstructured.Object

	metadata, _ := exchange["metadata"].(map[string]interface{})
	spec, _ := exchange["spec"].(map[string]interface{})
	status, _ := exchange["status"].(map[string]interface{})

	uid, _ := metadata["uid"].(string)
	phase, _ := status["phase"].(string)
	message, _ := status["message"].(string)
	streamName, _ := status["streamName"].(string)

	// Get subjects from spec or generate defaults
	var inputSubject, outputSubject string
	if streamSpec, ok := spec["stream"].(map[string]interface{}); ok {
		if subjects, ok := streamSpec["subjects"].(map[string]interface{}); ok {
			inputSubject, _ = subjects["input"].(string)
			outputSubject, _ = subjects["output"].(string)
		}
	}

	if inputSubject == "" {
		inputSubject = fmt.Sprintf("exchange.%s.input", uid)
	}
	if outputSubject == "" {
		outputSubject = fmt.Sprintf("exchange.%s.output", uid)
	}

	return &ExchangeResource{
		Name:      name,
		Namespace: namespace,
		UID:       uid,
		Phase:     phase,
		Message:   message,
		ConnectionInfo: ConnectionInfo{
			NATSURL:       c.natsURL,
			StreamName:    streamName,
			InputSubject:  inputSubject,
			OutputSubject: outputSubject,
		},
	}, nil
}

func (c *Conduit) waitForExchangeReady(name, namespace string, timeout time.Duration) (*ExchangeResource, error) {
	deadline := time.Now().Add(timeout)

	for time.Now().Before(deadline) {
		resource, err := c.getExchange(name, namespace)
		if err != nil {
			return nil, err
		}

		if resource.Phase == "Running" {
			return resource, nil
		}

		time.Sleep(2 * time.Second)
	}

	return nil, fmt.Errorf("Exchange %s/%s did not become ready within %v", namespace, name, timeout)
}

func (c *Conduit) deleteExchange(name, namespace string) error {
	// Define the GVR for Exchange CRD
	gvr := schema.GroupVersionResource{
		Group:    "conduit.mnke.org",
		Version:  "v1alpha1",
		Resource: "exchanges",
	}

	// Delete the Exchange CR
	err := c.dynamicClient.Resource(gvr).Namespace(namespace).Delete(
		context.Background(),
		name,
		metav1.DeleteOptions{},
	)
	if err != nil {
		return fmt.Errorf("failed to delete Exchange: %w", err)
	}

	return nil
}
