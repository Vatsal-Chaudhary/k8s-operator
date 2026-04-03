package controller

import (
	"fmt"
	"time"

	v1alpha1 "github.com/Vatsal-Chaudhary/k8s-operator/api/v1alpha1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

var _ = Describe("KafkaScalerReconciler", func() {
	AfterEach(func() {
		testLagFetcher.lag = 0
		testLagFetcher.err = nil
	})

	It("scales deployment up when lag exceeds threshold", func() {
		namespace := newTestNamespace("test-scale-up")
		Expect(k8sClient.Create(ctx, namespace)).To(Succeed())
		DeferCleanup(func() {
			Expect(k8sClient.Delete(ctx, namespace)).To(Succeed())
		})

		deployment := newDeployment(namespace.Name, "consumer", 2)
		Expect(k8sClient.Create(ctx, deployment)).To(Succeed())

		testLagFetcher.lag = 5000

		scaler := newKafkaScaler(namespace.Name, "scale-up", deployment.Name, 2, 20, 1000, 0)
		Expect(k8sClient.Create(ctx, scaler)).To(Succeed())

		deploymentKey := types.NamespacedName{Namespace: namespace.Name, Name: deployment.Name}
		Eventually(func(g Gomega) {
			fetched := &appsv1.Deployment{}
			g.Expect(k8sClient.Get(ctx, deploymentKey, fetched)).To(Succeed())
			g.Expect(fetched.Spec.Replicas).NotTo(BeNil())
			g.Expect(*fetched.Spec.Replicas).To(Equal(int32(5)))
		}, "10s", "250ms").Should(Succeed())

		scalerKey := types.NamespacedName{Namespace: namespace.Name, Name: scaler.Name}
		Eventually(func(g Gomega) {
			fetched := &v1alpha1.KafkaScaler{}
			g.Expect(k8sClient.Get(ctx, scalerKey, fetched)).To(Succeed())
			g.Expect(fetched.Status.CurrentLag).To(Equal(int64(5000)))
			g.Expect(fetched.Status.CurrentReplicas).To(Equal(int32(5)))
		}, "10s", "250ms").Should(Succeed())
	})

	It("respects cooldown and does not scale within cooldown window", func() {
		namespace := newTestNamespace("test-cooldown")
		Expect(k8sClient.Create(ctx, namespace)).To(Succeed())
		DeferCleanup(func() {
			Expect(k8sClient.Delete(ctx, namespace)).To(Succeed())
		})

		deployment := newDeployment(namespace.Name, "consumer", 3)
		Expect(k8sClient.Create(ctx, deployment)).To(Succeed())

		testLagFetcher.lag = 3000

		scaler := newKafkaScaler(namespace.Name, "cooldown", deployment.Name, 2, 20, 1000, 120)
		Expect(k8sClient.Create(ctx, scaler)).To(Succeed())

		scalerKey := types.NamespacedName{Namespace: namespace.Name, Name: scaler.Name}
		Eventually(func(g Gomega) {
			fetched := &v1alpha1.KafkaScaler{}
			g.Expect(k8sClient.Get(ctx, scalerKey, fetched)).To(Succeed())
			g.Expect(fetched.Status.CurrentReplicas).To(Equal(int32(3)))
			g.Expect(fetched.Status.LastScaleTime).NotTo(BeNil())
		}, "10s", "250ms").Should(Succeed())

		testLagFetcher.lag = 8000
		Eventually(func(g Gomega) {
			fetched := &v1alpha1.KafkaScaler{}
			g.Expect(k8sClient.Get(ctx, scalerKey, fetched)).To(Succeed())
			now := metav1.NewTime(time.Now().Add(-30 * time.Second))
			fetched.Status.CurrentReplicas = 3
			fetched.Status.LastScaleTime = &now
			g.Expect(k8sClient.Status().Update(ctx, fetched)).To(Succeed())
		}, "10s", "250ms").Should(Succeed())

		deploymentKey := types.NamespacedName{Namespace: namespace.Name, Name: deployment.Name}
		Consistently(func(g Gomega) {
			fetched := &appsv1.Deployment{}
			g.Expect(k8sClient.Get(ctx, deploymentKey, fetched)).To(Succeed())
			g.Expect(fetched.Spec.Replicas).NotTo(BeNil())
			g.Expect(*fetched.Spec.Replicas).To(Equal(int32(3)))
		}, "3s", "250ms").Should(Succeed())
	})

	It("sets deployment to min replicas when lag is zero", func() {
		namespace := newTestNamespace("test-scale-down")
		Expect(k8sClient.Create(ctx, namespace)).To(Succeed())
		DeferCleanup(func() {
			Expect(k8sClient.Delete(ctx, namespace)).To(Succeed())
		})

		deployment := newDeployment(namespace.Name, "consumer", 10)
		Expect(k8sClient.Create(ctx, deployment)).To(Succeed())

		testLagFetcher.lag = 0

		scaler := newKafkaScaler(namespace.Name, "scale-down", deployment.Name, 2, 20, 1000, 0)
		Expect(k8sClient.Create(ctx, scaler)).To(Succeed())

		deploymentKey := types.NamespacedName{Namespace: namespace.Name, Name: deployment.Name}
		Eventually(func(g Gomega) {
			fetched := &appsv1.Deployment{}
			g.Expect(k8sClient.Get(ctx, deploymentKey, fetched)).To(Succeed())
			g.Expect(fetched.Spec.Replicas).NotTo(BeNil())
			g.Expect(*fetched.Spec.Replicas).To(Equal(int32(2)))
		}, "10s", "250ms").Should(Succeed())
	})
})

func newTestNamespace(prefix string) *corev1.Namespace {
	return &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: fmt.Sprintf("%s-%d", prefix, time.Now().UnixNano()),
		},
	}
}

func newDeployment(namespace string, name string, replicas int32) *appsv1.Deployment {
	labels := map[string]string{"app": name}

	return &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &replicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: labels,
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: labels,
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{{
						Name:  "consumer",
						Image: "nginx:1.25",
					}},
				},
			},
		},
	}
}

func newKafkaScaler(
	namespace string,
	name string,
	deploymentName string,
	minReplicas int32,
	maxReplicas int32,
	lagPerReplica int64,
	cooldownSeconds int,
) *v1alpha1.KafkaScaler {
	return &v1alpha1.KafkaScaler{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: v1alpha1.KafkaScalerSpec{
			Topic:           "orders",
			ConsumerGroup:   "orders-consumer",
			DeploymentName:  deploymentName,
			TargetNamespace: namespace,
			MinReplicas:     minReplicas,
			MaxReplicas:     maxReplicas,
			LagPerReplica:   lagPerReplica,
			CooldownSeconds: cooldownSeconds,
			KafkaBrokers:    []string{"localhost:9092"},
		},
	}
}
