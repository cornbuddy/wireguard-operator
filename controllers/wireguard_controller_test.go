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
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	vpnv1alpha1 "github.com/ahova-vpn/wireguard-operator/api/v1alpha1"
)

const (
	timeout  = 10 * time.Second
	interval = 200 * time.Millisecond
)

var _ = Describe("Wireguard controller", func() {
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

	AfterEach(func() {
		By("deleting Wireguard resources")
		wireguard := vpnv1alpha1.Wireguard{}
		err := k8sClient.DeleteAllOf(context.TODO(), &wireguard)
		deletedOrNotFound := err == nil || apierrors.IsNotFound(err)
		Expect(deletedOrNotFound).To(BeTrue())
	})

	DescribeTable("should reconcile successfully",
		func(wireguard *vpnv1alpha1.Wireguard) {
			testReconcile(wireguard)
		},
		Entry(
			"default configuration",
			&vpnv1alpha1.Wireguard{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "wireguard-default",
					Namespace: corev1.NamespaceDefault,
				},
				Spec: vpnv1alpha1.WireguardSpec{},
			},
		),
		Entry(
			"internal DNS configuration",
			&vpnv1alpha1.Wireguard{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "wireguard-internal-dns",
					Namespace: corev1.NamespaceDefault,
				},
				Spec: vpnv1alpha1.WireguardSpec{
					ExternalDNS: vpnv1alpha1.ExternalDNS{
						Enabled: false,
					},
				},
			},
		),
		Entry(
			"configuration with sidecars",
			&vpnv1alpha1.Wireguard{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "wireguard-sidecars",
					Namespace: corev1.NamespaceDefault,
				},
				Spec: vpnv1alpha1.WireguardSpec{
					Sidecars: []corev1.Container{sidecar},
				},
			},
		),
		Entry(
			"pre-configured public key",
			&vpnv1alpha1.Wireguard{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "wireguard-public-key",
					Namespace: corev1.NamespaceDefault,
				},
				Spec: vpnv1alpha1.WireguardSpec{
					PeerPublicKey: toPtr(key.PublicKey().String()),
				},
			},
		),
	)
})

// Performs full reconcildation loop for wireguard
func reconcileWireguard(ctx context.Context, key types.NamespacedName) error {
	reconciler := &WireguardReconciler{
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

// Validates Wireguard resource and all dependent resources
func testReconcile(wireguard *vpnv1alpha1.Wireguard) {
	GinkgoHelper()

	By("Setting prerequisites")
	key := types.NamespacedName{
		Name:      wireguard.ObjectMeta.Name,
		Namespace: wireguard.ObjectMeta.Namespace,
	}
	ctx := context.Background()

	By("Creating the custom resource for the Kind Wireguard")
	Expect(k8sClient.Create(ctx, wireguard)).To(Succeed())

	By("Checking if the custom resource was successfully created")
	Eventually(func() error {
		found := &vpnv1alpha1.Wireguard{}
		return k8sClient.Get(ctx, key, found)
	}, timeout, interval).Should(Succeed())

	By("Reconciling the custom resource created")
	Expect(reconcileWireguard(ctx, key)).To(Succeed())

	By("Checking if ConfigMap was successfully created in the reconciliation")
	Eventually(func() error {
		found := &corev1.ConfigMap{}
		return k8sClient.Get(ctx, key, found)
	}, timeout, interval).Should(Succeed())

	By("Checking if Secret was successfully created in the reconciliation")
	Eventually(func() error {
		secret := &corev1.Secret{}
		if err := k8sClient.Get(ctx, key, secret); err != nil {
			return err
		}
		Expect(secret.Data).To(HaveKey("wg-server"))
		if wireguard.Spec.PeerPublicKey == nil {
			Expect(secret.Data).To(HaveKey("wg-client"))
		} else {
			Expect(secret.Data).To(Not(HaveKey("wg-client")))
		}

		peerAddr := getLastIpInSubnet(wireguard.Spec.Address)
		masquerade := fmt.Sprintf(
			"PostUp = iptables --table nat --append POSTROUTING --source %s --out-interface eth0 --jump MASQUERADE",
			peerAddr,
		)
		mandatoryPostUps := []string{
			"PostUp = iptables --append FORWARD --in-interface %i --jump ACCEPT",
			"PostUp = iptables --append FORWARD --out-interface %i --jump ACCEPT",
			masquerade,
		}
		hardeningPostUps := getHardeningPostUps(wireguard)
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
		sidecarsLen := len(wireguard.Spec.Sidecars)
		if wireguard.Spec.ExternalDNS.Enabled {
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
		conditions := wireguard.Status.Conditions
		if conditions != nil && len(conditions) != 0 {
			got := conditions[len(conditions)-1]
			msg := fmt.Sprintf(
				"Deployment for custom resource (%s) with %d replicas created successfully",
				wireguard.Name, wireguard.Spec.Replicas)
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

var _ = DescribeTable("getLastIpInSubnet",
	func(input, want string) {
		got := getLastIpInSubnet(input)
		Expect(got).To(Equal(want))
	},
	Entry("smol", "192.168.254.253/30", "192.168.254.254/32"),
	Entry("chungus", "192.168.1.1/24", "192.168.1.254/32"),
)

var _ = DescribeTable("getFirstIpInSubnet",
	func(input, want string) {
		got := getFirstIpInSubnet(input)
		Expect(got).To(Equal(want))
	},
	Entry("smol", "192.168.254.253/30", "192.168.254.253/32"),
	Entry("chungus", "192.168.1.1/24", "192.168.1.1/32"),
)

func getHardeningPostUps(wireguard *vpnv1alpha1.Wireguard) []string {
	var postUps []string
	peerAddress := getLastIpInSubnet(wireguard.Spec.Address)
	for _, dest := range wireguard.Spec.DropConnectionsTo {
		postUp := fmt.Sprintf(
			"PostUp = iptables --insert FORWARD --source %s --destination %s --jump DROP",
			peerAddress,
			dest,
		)
		postUps = append(postUps, postUp)
	}
	return postUps
}
