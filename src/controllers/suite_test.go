package controllers

import (
	"context"
	"log"
	"os"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/cornbuddy/wireguard-operator/src/api/v1alpha1"
	"github.com/cornbuddy/wireguard-operator/src/test/dsl"
	"github.com/cornbuddy/wireguard-operator/src/test/testenv"
	//+kubebuilder:scaffold:imports
)

const (
	timeout       = 30 * time.Second
	tick          = 1 * time.Second
	wireguardPort = 51820
)

var (
	k8sClient client.Client
	wgDsl     dsl.Dsl
	peerDsl   dsl.Dsl
	ctx       = context.TODO()
)

func TestMain(m *testing.M) {
	config, err := testenv.Setup()
	if err != nil {
		log.Fatalf("failed to setup test env: %v", err)
	}

	err = v1alpha1.AddToScheme(scheme.Scheme)
	if err != nil {
		log.Fatalf("failed to setup scheme: %v", err)
	}

	//+kubebuilder:scaffold:scheme

	k8sClient, err = client.New(config, client.Options{Scheme: scheme.Scheme})
	if err != nil {
		log.Fatalf("failed to setup k8s client: %v", err)
	}

	peerDsl = dsl.Dsl{
		K8sClient: k8sClient,
		Reconciler: &WireguardPeerReconciler{
			Client: k8sClient,
			Scheme: k8sClient.Scheme(),
		},
	}
	wgDsl = dsl.Dsl{
		K8sClient: k8sClient,
		Reconciler: &WireguardReconciler{
			Client: k8sClient,
			Scheme: k8sClient.Scheme(),
		},
	}

	os.Exit(m.Run())
}

func extractClusterIp(_ v1alpha1.Wireguard, svc corev1.Service) string {
	return svc.Spec.ClusterIP
}

func extractWireguardEndpoint(wg v1alpha1.Wireguard, _ corev1.Service) string {
	return *wg.Spec.EndpointAddress
}

func extractFromStatus(wg v1alpha1.Wireguard, _ corev1.Service) string {
	if wg.Status.Endpoint == nil {
		return "wtf????"
	}

	return *wg.Status.Endpoint
}
