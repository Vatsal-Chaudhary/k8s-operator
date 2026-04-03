package controller

import (
	"context"
	"path/filepath"
	"testing"

	v1alpha1 "github.com/Vatsal-Chaudhary/k8s-operator/api/v1alpha1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
)

var (
	ctx            context.Context
	cancel         context.CancelFunc
	k8sClient      client.Client
	testEnv        *envtest.Environment
	testLagFetcher *mockLagFetcher
)

type mockLagFetcher struct {
	lag int64
	err error
}

func (m *mockLagFetcher) GetConsumerLag(
	ctx context.Context,
	brokers []string,
	topic string,
	consumerGroup string,
) (int64, error) {
	return m.lag, m.err
}

func TestController(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Controller Suite")
}

var _ = BeforeSuite(func() {
	logf.SetLogger(zap.New(zap.WriteTo(GinkgoWriter), zap.UseDevMode(true)))

	ctx, cancel = context.WithCancel(context.TODO())
	testEnv = &envtest.Environment{
		CRDDirectoryPaths: []string{
			filepath.Join("..", "..", "config", "crd", "bases"),
		},
		ErrorIfCRDPathMissing: true,
	}

	cfg, err := testEnv.Start()
	Expect(err).NotTo(HaveOccurred())
	Expect(cfg).NotTo(BeNil())

	Expect(v1alpha1.AddToScheme(scheme.Scheme)).To(Succeed())
	Expect(appsv1.AddToScheme(scheme.Scheme)).To(Succeed())

	k8sClient, err = client.New(cfg, client.Options{Scheme: scheme.Scheme})
	Expect(err).NotTo(HaveOccurred())
	Expect(k8sClient).NotTo(BeNil())

	mgr, err := ctrl.NewManager(cfg, ctrl.Options{Scheme: scheme.Scheme})
	Expect(err).NotTo(HaveOccurred())

	testLagFetcher = &mockLagFetcher{lag: 0}
	err = (&KafkaScalerReconciler{
		Client:     mgr.GetClient(),
		Scheme:     mgr.GetScheme(),
		LagFetcher: testLagFetcher,
	}).SetupWithManager(mgr)
	Expect(err).NotTo(HaveOccurred())

	go func() {
		defer GinkgoRecover()
		Expect(mgr.Start(ctx)).To(Succeed())
	}()
})

var _ = AfterSuite(func() {
	if cancel != nil {
		cancel()
	}

	if testEnv != nil {
		Expect(testEnv.Stop()).To(Succeed())
	}
})
