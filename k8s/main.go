package k8s

import (
	"context"
	"fmt"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/httpstream"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/transport/spdy"
	"math/rand"
	"net/http"
	"strconv"
	"strings"
)

type TargetPod struct {
	Name      string
	Port      string
	Namespace string
}

type Api struct {
	api  v1.CoreV1Interface
	conf *rest.Config
}

func New(kubeconfig string) (*Api, error) {
	kconf, err := clientcmd.BuildConfigFromFlags("", kubeconfig)
	if err != nil {
		return nil, fmt.Errorf("could not load k8s config: %w", err)
	}

	// create the clientset
	clientset, err := kubernetes.NewForConfig(kconf)
	if err != nil {
		return nil, fmt.Errorf("could not load k8s clientset: %w", err)
	}

	apiInstance := clientset.CoreV1()
	api := &Api{
		api:  apiInstance,
		conf: kconf,
	}

	return api, nil
}

func (api *Api) GetMatchingPod(ctx context.Context, namespace, podName, port string) (*TargetPod, error) {
	pod, err := api.api.Pods(namespace).Get(ctx, podName, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("could not find pods: %w", err)
	}
	return &TargetPod{
		Namespace: pod.Namespace,
		Name:      pod.Name,
		Port:      port,
	}, nil
}

func (api *Api) GetMatchingPodForService(ctx context.Context, namespace, serviceName, port string) (*TargetPod, error) {
	svc, err := api.api.Services(namespace).Get(ctx, serviceName, metav1.GetOptions{})
	if err != nil {
		panic(err)
	}

	var podPort string
	for _, p := range svc.Spec.Ports {
		pp := strconv.Itoa(int(p.Port))
		if pp == port || p.Name == port {
			podPort = p.TargetPort.String()
			break
		}
	}
	var selectors []string
	for k, v := range svc.Spec.Selector {
		selectors = append(selectors, fmt.Sprintf("%s=%s", k, v))
	}

	pods, err := api.api.Pods(namespace).List(ctx, metav1.ListOptions{LabelSelector: strings.Join(selectors, ",")})
	if err != nil {
		return nil, fmt.Errorf("could not find pods: %w", err)
	}
	if len(pods.Items) == 0 {
		return nil, fmt.Errorf("no pods found")
	}

	randIdx := rand.Intn(len(pods.Items))
	randPod := pods.Items[randIdx]

	return &TargetPod{
		Namespace: randPod.Namespace,
		Name:      randPod.Name,
		Port:      podPort,
	}, nil
}

func (api *Api) Dialer(p *TargetPod) (httpstream.Dialer, error) {
	transport, upgrader, err := spdy.RoundTripperFor(api.conf)
	if err != nil {
		return nil, fmt.Errorf("could not create spdy round tripper: %w", err)
	}
	url := api.api.RESTClient().Post().
		Resource("pods").
		Namespace(p.Namespace).
		Name(p.Name).
		SubResource("portforward").URL()
	dialer := spdy.NewDialer(upgrader, &http.Client{Transport: transport}, "POST", url)
	return dialer, nil
}
