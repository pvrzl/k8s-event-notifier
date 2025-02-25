package controller

import (
	"context"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	monitoringv1 "github.com/example/notifier/api/v1"
)

var _ = Describe("Notifier Controller", func() {
	Context("When a Kubernetes event occurs", func() {
		const (
			resourceName  = "test-notifier-resource"
			testNamespace = "default"
			testPodName   = "failing-pod"
		)

		ctx := context.Background()
		typeNamespacedName := types.NamespacedName{Name: resourceName, Namespace: testNamespace}
		notifier := &monitoringv1.Notifier{}

		BeforeEach(func() {
			By("Creating the Notifier custom resource")
			err := k8sClient.Get(ctx, typeNamespacedName, notifier)
			if errors.IsNotFound(err) {
				resource := &monitoringv1.Notifier{
					ObjectMeta: metav1.ObjectMeta{Name: resourceName, Namespace: testNamespace},
					Spec: monitoringv1.NotifierSpec{
						Namespaces:   []string{"default"},
						EventTypes:   []string{"Warning"},
						EventReasons: []string{"ImagePullFailed"},
						Channel:      monitoringv1.Slack,
						Webhook:      "https://hooks.slack.com/services/test/webhook",
					},
				}
				Expect(k8sClient.Create(ctx, resource)).To(Succeed())
			}
		})

		AfterEach(func() {
			By("Cleaning up the Notifier resource")
			resource := &monitoringv1.Notifier{}
			err := k8sClient.Get(ctx, typeNamespacedName, resource)
			if err == nil {
				Expect(k8sClient.Delete(ctx, resource)).To(Succeed())
			}
		})

		It("should detect an ImagePullFailed event and process it", func() {
			By("Creating a failing Pod")
			failingPod := &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{Name: testPodName, Namespace: testNamespace},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{{
						Name:  "test-container",
						Image: "invalid-image-name:latest", // This should fail to pull
					}},
				},
			}
			Expect(k8sClient.Create(ctx, failingPod)).To(Succeed())

			By("Simulating a Kubernetes event for ImagePullFailed")
			fakeEvent := &corev1.Event{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-event",
					Namespace: testNamespace,
				},
				InvolvedObject: corev1.ObjectReference{
					Kind:      "Pod",
					Namespace: testNamespace,
					Name:      testPodName,
				},
				Reason:        "ImagePullFailed",
				Type:          "Warning",
				Message:       "Failed to pull image 'invalid-image-name:latest'",
				LastTimestamp: metav1.NewTime(time.Now()),
			}
			Expect(k8sClient.Create(ctx, fakeEvent)).To(Succeed())

			By("Triggering reconciliation")
			controllerReconciler := &NotifierReconciler{
				Client: k8sClient,
				Scheme: k8sClient.Scheme(),
			}

			_, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: typeNamespacedName,
			})
			Expect(err).NotTo(HaveOccurred())

			By("Verifying that the event was processed")
			Eventually(func() bool {
				err := k8sClient.Get(ctx, typeNamespacedName, notifier)
				if err != nil {
					return false
				}
				// Check if Notifier status has been updated
				return notifier.Status.LastEventTime != nil
			}, time.Second*5, time.Millisecond*500).Should(BeTrue())
		})
	})
})
