package controllers

import (
	"context"
	"fmt"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	wgtypes "golang.zx2c4.com/wireguard/wgctrl/wgtypes"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	vpnv1alpha1 "github.com/ahova-vpn/wireguard-operator/api/v1alpha1"
)

const (
	timeout  = 10 * time.Second
	interval = 200 * time.Millisecond
)

var _ = Describe("WireguardPeer controller", func() {
	sidecar := corev1.Container{
		Name:  "wireguard-exporter",
		Image: "docker.io/mindflavor/prometheus-wireguard-exporter:3.6.6",
		Args: []string{
			"--verbose", "true",
			"--extract_names_config_files", "/config/wg0.conf",
		},
		VolumeMounts: []corev1.VolumeMount{{
			Name:      "wireguard-config",
			ReadOnly:  true,
			MountPath: "/config",
		}},
		SecurityContext: &corev1.SecurityContext{
			RunAsUser:  toPtr[int64](0),
			RunAsGroup: toPtr[int64](0),
			Capabilities: &corev1.Capabilities{
				Add: []corev1.Capability{
					"NET_ADMIN",
				},
			},
		},
	}
	key, _ := wgtypes.GeneratePrivateKey()

	BeforeEach(func() {
		By("Creating wireguard CR")
		wireguard := &vpnv1alpha1.Wireguard{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "wireguard",
				Namespace: corev1.NamespaceDefault,
			},
			Spec: vpnv1alpha1.WireguardSpec{},
		}
		Expect(createIfNotExists(wireguard)).To(Succeed())

		By("Checking if wireguard CR was successfully created")
		key := types.NamespacedName{
			Name:      wireguard.ObjectMeta.Name,
			Namespace: wireguard.ObjectMeta.Namespace,
		}
		Eventually(func() error {
			return k8sClient.Get(context.TODO(), key, wireguard)
		}, timeout, interval).Should(Succeed())
	})

	AfterEach(func() {
		By("Deleting wireguard peers CR")
		peer := &vpnv1alpha1.WireguardPeer{}
		err := k8sClient.DeleteAllOf(context.TODO(), peer)
		deletedOrNotFound := err == nil || apierrors.IsNotFound(err)
		Expect(deletedOrNotFound).To(BeTrue())
	})

	DescribeTable("should reconcile successfully",
		testReconcile,
		Entry(
			"default configuration",
			&vpnv1alpha1.WireguardPeer{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "wireguard-default",
					Namespace: corev1.NamespaceDefault,
				},
				Spec: vpnv1alpha1.WireguardPeerSpec{
					WireguardRef: "wireguard",
				},
			},
		),
		Entry(
			"internal DNS configuration",
			&vpnv1alpha1.WireguardPeer{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "wireguard-internal-dns",
					Namespace: corev1.NamespaceDefault,
				},
				Spec: vpnv1alpha1.WireguardPeerSpec{
					WireguardRef: "wireguard",
					ExternalDNS: vpnv1alpha1.ExternalDNS{
						Enabled: false,
					},
				},
			},
		),
		Entry(
			"configuration with sidecars",
			&vpnv1alpha1.WireguardPeer{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "wireguard-sidecars",
					Namespace: corev1.NamespaceDefault,
				},
				Spec: vpnv1alpha1.WireguardPeerSpec{
					WireguardRef: "wireguard",
					Sidecars:     []corev1.Container{sidecar},
				},
			},
		),
		Entry(
			"pre-configured public key",
			&vpnv1alpha1.WireguardPeer{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "wireguard-public-key",
					Namespace: corev1.NamespaceDefault,
				},
				Spec: vpnv1alpha1.WireguardPeerSpec{
					WireguardRef: "wireguard",
					PublicKey:    toPtr(key.PublicKey().String()),
				},
			},
		),
	)
})

// Validates reconcilation of peer. Creates WireguardPeer CR as a side effect
func validateReconcile(peer *vpnv1alpha1.WireguardPeer) {
	GinkgoHelper()

	key := types.NamespacedName{
		Name:      peer.ObjectMeta.Name,
		Namespace: peer.ObjectMeta.Namespace,
	}

	By("Creating the custom resource for the Kind Wireguard")
	Expect(k8sClient.Create(context.TODO(), peer)).To(Succeed())

	By("Checking if the custom resource was successfully created")
	Eventually(func() error {
		return k8sClient.Get(context.TODO(), key, &vpnv1alpha1.WireguardPeer{})
	}, timeout, interval).Should(Succeed())

	By("Reconciling the custom resource created")
	Expect(reconcileWireguard(context.TODO(), key)).To(Succeed())
}

func validateConfigMap(peer *vpnv1alpha1.WireguardPeer) {
	GinkgoHelper()

	key := types.NamespacedName{
		Name:      peer.ObjectMeta.Name,
		Namespace: peer.ObjectMeta.Namespace,
	}

	By("Checking if ConfigMap was successfully created in the reconciliation")
	Eventually(func() error {
		found := &corev1.ConfigMap{}
		return k8sClient.Get(context.TODO(), key, found)
	}, timeout, interval).Should(Succeed())
}

// Validates WireguardPeer resource and all dependent resources
func testReconcile(peer *vpnv1alpha1.WireguardPeer) {
	GinkgoHelper()

	// TODO: move this logic into BeforeEach block
	validateReconcile(peer)

	validateConfigMap(peer)

	// TODO: make as above and then move to the describe table body

	By("Setting prerequisites")
	key := types.NamespacedName{
		Name:      peer.ObjectMeta.Name,
		Namespace: peer.ObjectMeta.Namespace,
	}
	ctx := context.TODO()

	By("Checking if Secret was successfully created in the reconciliation")
	Eventually(func() error {
		secret := &corev1.Secret{}
		if err := k8sClient.Get(ctx, key, secret); err != nil {
			return err
		}
		Expect(secret.Data).To(HaveKey("wg-server"))
		if peer.Spec.PublicKey == nil {
			Expect(secret.Data).To(HaveKey("wg-client"))
		} else {
			Expect(secret.Data).To(Not(HaveKey("wg-client")))
		}

		masquerade := fmt.Sprintf(
			"PostUp = iptables --table nat --append POSTROUTING --source %s --out-interface eth0 --jump MASQUERADE",
			peer.Spec.Address,
		)
		mandatoryPostUps := []string{
			"PostUp = iptables --append FORWARD --in-interface %i --jump ACCEPT",
			"PostUp = iptables --append FORWARD --out-interface %i --jump ACCEPT",
			masquerade,
		}
		hardeningPostUps := getHardeningPostUps(peer)
		cfg := string(secret.Data["wg-server"])
		for _, postUp := range append(mandatoryPostUps, hardeningPostUps...) {
			Expect(cfg).To(ContainSubstring(postUp))
		}
		return nil
	}, timeout, interval).Should(Succeed())

	By("Checking if Deployment was successfully created in the reconciliation")
	Eventually(func() error {
		deploy := &appsv1.Deployment{}
		err := k8sClient.Get(ctx, key, deploy)
		if err != nil {
			return err
		}

		context := &corev1.SecurityContext{
			Privileged: toPtr(true),
			Capabilities: &corev1.Capabilities{
				Add: []corev1.Capability{
					"NET_ADMIN",
					"SYS_MODULE",
				},
			},
		}
		containers := deploy.Spec.Template.Spec.Containers
		wg := containers[0]
		Expect(wg.SecurityContext).To(BeEquivalentTo(context))

		gotSysctls := deploy.Spec.Template.Spec.SecurityContext.Sysctls
		wantSysctls := []corev1.Sysctl{{
			Name:  "net.ipv4.ip_forward",
			Value: "1",
		}}
		Expect(gotSysctls).To(BeEquivalentTo(wantSysctls))

		dnsConfig := deploy.Spec.Template.Spec.DNSConfig
		dnsPolicy := deploy.Spec.Template.Spec.DNSPolicy
		volumes := deploy.Spec.Template.Spec.Volumes
		sidecarsLen := len(peer.Spec.Sidecars)
		if peer.Spec.ExternalDNS.Enabled {
			Expect(len(containers)).To(Equal(2 + sidecarsLen))
			Expect(len(volumes)).To(Equal(2))
			want := &corev1.PodDNSConfig{
				Nameservers: []string{"127.0.0.1"},
			}
			Expect(dnsConfig).To(BeEquivalentTo(want))
			Expect(dnsPolicy).To(Equal(corev1.DNSNone))
		} else {
			Expect(len(containers)).To(Equal(1 + sidecarsLen))
			Expect(len(volumes)).To(Equal(1))
			want := &corev1.PodDNSConfig{}
			Expect(dnsConfig).To(BeEquivalentTo(want))
			Expect(dnsPolicy).To(Equal(corev1.DNSDefault))
		}

		return nil
	}, timeout, interval).Should(Succeed())

	By("Checking if Service was successfully created in the reconciliation")
	Eventually(func() error {
		found := &corev1.Service{}
		return k8sClient.Get(ctx, key, found)
	}, timeout, interval).Should(Succeed())

	By("Checking the latest Status Condition added to the Wireguard instance")
	Eventually(func() error {
		conditions := peer.Status.Conditions
		conditionsNotEmpty := conditions != nil && len(conditions) != 0
		if conditionsNotEmpty {
			got := conditions[len(conditions)-1]
			msg := fmt.Sprintf(
				"Deployment for custom resource (%s) with %d replicas created successfully",
				peer.Name, peer.Spec.Replicas)
			want := metav1.Condition{
				Type:    typeAvailableWireguard,
				Status:  metav1.ConditionTrue,
				Reason:  "Reconciling",
				Message: msg,
			}
			if got != want {
				return fmt.Errorf("The latest status condition added to the wireguard instance is not as expected")
			}
		}
		return nil
	}, timeout, interval).Should(Succeed())
}

// Performs full reconcildation loop for wireguard
func reconcileWireguard(ctx context.Context, key types.NamespacedName) error {
	reconciler := &WireguardPeerReconciler{
		Client: k8sClient,
		Scheme: k8sClient.Scheme(),
	}
	// Reconcile resource multiple times to ensure that all resources are
	// created
	for i := 0; i < 5; i++ {
		req := reconcile.Request{
			NamespacedName: key,
		}
		if _, err := reconciler.Reconcile(ctx, req); err != nil {
			return err
		}
	}
	return nil
}

func createIfNotExists(obj client.Object) error {
	key := types.NamespacedName{
		Name:      obj.GetName(),
		Namespace: obj.GetNamespace(),
	}

	err := k8sClient.Get(context.TODO(), key, obj)
	if err != nil && !apierrors.IsNotFound(err) {
		return err
	} else if err == nil {
		return nil
	}

	if err := k8sClient.Create(context.TODO(), obj); err != nil {
		return err
	}
	return nil
}

func getHardeningPostUps(peer *vpnv1alpha1.WireguardPeer) []string {
	var postUps []string
	for _, dest := range peer.Spec.DropConnectionsTo {
		postUp := fmt.Sprintf(
			"PostUp = iptables --insert FORWARD --source %s --destination %s --jump DROP",
			peer.Spec.Address,
			dest,
		)
		postUps = append(postUps, postUp)
	}
	return postUps
}

var _ = DescribeTable("getFirstIpInSubnet",
	func(input, want string) {
		got := getFirstIpInSubnet(input)
		Expect(got).To(Equal(want))
	},
	Entry("smol", "192.168.254.253/30", "192.168.254.253/32"),
	Entry("chungus", "192.168.1.1/24", "192.168.1.1/32"),
)
