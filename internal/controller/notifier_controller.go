/*
Copyright 2025.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controller

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"golang.org/x/time/rate"
	corev1 "k8s.io/api/core/v1"

	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"

	monitoringv1 "github.com/example/notifier/api/v1"

	"github.com/example/notifier/pkg/publisher"
	"github.com/example/notifier/pkg/publisher/slack"
)

// ? should we move to config?
// Define a rate limiter (allow max 1 event per second, with burst of 5)
var eventRateLimiter = rate.NewLimiter(rate.Limit(1), 10)

// NotifierReconciler reconciles a Notifier object
type NotifierReconciler struct {
	client.Client
	Scheme          *runtime.Scheme
	processedEvents sync.Map
}

type NotifierConfig struct {
	Namespaces       map[string]bool
	EventTypes       map[string]bool
	EventReasons     map[string]bool
	EventObjectTypes map[string]bool
	MessageContains  []string
}

// +kubebuilder:rbac:groups="",resources=events,verbs=get;list;watch
// +kubebuilder:rbac:groups=monitoring.example.com,resources=notifiers,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=monitoring.example.com,resources=notifiers/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=monitoring.example.com,resources=notifiers/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.

// the Notifier object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.20.0/pkg/reconcile
func (r *NotifierReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	var (
		log             = log.FromContext(ctx)
		notifiers       monitoringv1.NotifierList
		eventList       corev1.EventList
		processedEvents []string
	)

	log.Info("Starting reconciliation process")

	if err := r.List(ctx, &notifiers); err != nil {
		log.Error(err, "Failed to list Notifier CRs")
		return ctrl.Result{}, err
	}

	if err := r.List(ctx, &eventList); err != nil {
		log.Error(err, "failed to list events")
		return ctrl.Result{}, err
	}

	for _, notifier := range notifiers.Items {
		publisher, err := r.publisherFactory(ctx, &notifier)
		if err != nil {
			log.Error(err, "failed to create publisher")
			return ctrl.Result{}, err
		}

		for _, k8sEvent := range eventList.Items {
			stringEvents := fmt.Sprintf("%+v", k8sEvent)
			if r.shouldNotify(ctx, &notifier, k8sEvent) {
				if err := eventRateLimiter.Wait(ctx); err != nil {
					log.Error(err, "Rate limiting failed")
					continue
				}

				message := r.constructEventMessage(ctx, &notifier, k8sEvent)
				r.logVerbose(ctx, &notifier, "will send "+stringEvents)
				err := publisher.Send(ctx, message)
				if err != nil {
					log.Error(err, "failed to send webhook")
					continue
				}

				processedEvents = append(processedEvents, fmt.Sprintf("%s: %s", k8sEvent.Reason, k8sEvent.Message))
			} else {
				r.logVerbose(ctx, &notifier, "not valid event"+stringEvents)
			}
		}

		if len(processedEvents) > 0 {
			notifier.Status.LastEventTime = &eventList.Items[len(eventList.Items)-1].LastTimestamp
			notifier.Status.RecentEvents = processedEvents
			notifier.Status.StatusMessage = fmt.Sprintf("Processed %d events", len(processedEvents))

			if err := r.Status().Update(ctx, &notifier); err != nil {
				log.Error(err, "failed to update notifier status")
				return ctrl.Result{}, err
			}
		}
	}

	log.Info("Reconciliation successful")
	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *NotifierReconciler) SetupWithManager(mgr ctrl.Manager) error {
	r.startCleanupRoutine()
	return ctrl.NewControllerManagedBy(mgr).
		For(&monitoringv1.Notifier{}).
		Watches(
			&corev1.Event{},
			&handler.EnqueueRequestForObject{}).
		Named("notifier").
		Complete(r)
}

func (r *NotifierReconciler) logVerbose(ctx context.Context, notifier *monitoringv1.Notifier, msg string, keysAndValues ...any) {
	log := log.FromContext(ctx)
	if notifier.Spec.DefaultSettings != nil && notifier.Spec.DefaultSettings.EnableVerbose {
		log.Info(msg, keysAndValues...)
	}
}

func (r *NotifierReconciler) publisherFactory(_ context.Context, notifier *monitoringv1.Notifier) (publisher.Publisher, error) {
	switch notifier.Spec.Channel {
	case monitoringv1.Slack:
		return slack.NewSlackPublisher(notifier.Spec.Webhook), nil
	default:
		return nil, fmt.Errorf("unsupported publisher channel: %s", notifier.Spec.Channel)
	}
}

func (r *NotifierReconciler) parseNotifierConfig(_ context.Context, notifier *monitoringv1.Notifier) *NotifierConfig {
	return &NotifierConfig{
		Namespaces:       toMap(notifier.Spec.Namespaces),
		EventTypes:       toMap(notifier.Spec.EventTypes),
		EventReasons:     toMap(notifier.Spec.EventReasons),
		EventObjectTypes: toMap(notifier.Spec.EventObjectTypes),
		MessageContains:  notifier.Spec.MessageContains,
	}
}

func (r *NotifierReconciler) shouldNotify(ctx context.Context, notifier *monitoringv1.Notifier, event corev1.Event) bool {
	config := r.parseNotifierConfig(ctx, notifier)

	if _, exists := r.processedEvents.Load(event.UID); exists {
		return false
	}

	if !config.Namespaces[event.Namespace] {
		return false
	}

	if !config.EventTypes[event.Type] {
		return false
	}

	if len(config.EventReasons) > 0 && !config.EventReasons[event.Reason] {
		return false
	}

	if len(config.EventObjectTypes) > 0 && !config.EventObjectTypes[event.InvolvedObject.Kind] {
		return false
	}

	for _, message := range config.MessageContains {
		if strings.Contains(
			strings.ToLower(event.Message),
			strings.ToLower(message),
		) {
			r.processedEvents.Store(event.UID, time.Now())
			return true
		}
	}

	if len(config.MessageContains) > 0 {
		return false
	}

	r.processedEvents.Store(event.UID, time.Now())
	return true
}

func (r *NotifierReconciler) startCleanupRoutine() {
	// ? should this moved to config?
	ticker := time.NewTicker(5 * time.Minute)
	go func() {
		for range ticker.C {
			now := time.Now()
			r.processedEvents.Range(func(key, value interface{}) bool {
				if t, ok := value.(time.Time); ok && now.Sub(t) > 5*time.Minute {
					r.processedEvents.Delete(key)
				}
				return true
			})
		}
	}()
}

func (r *NotifierReconciler) constructEventMessage(_ context.Context, notifier *monitoringv1.Notifier, event corev1.Event) string {
	// TODO: Introduce a template for better message formatting.
	prefix := ""
	settings := notifier.Spec.DefaultSettings
	if settings != nil {
		prefix = settings.MessagePrefix
	}

	return fmt.Sprintf("*%s*\n*%s* in namespace *%s*\n*Reason:* %s\n*Message:* %s\n*Affecting Object Type:* %s\n*Affecting Object Name:* %s",
		prefix,
		event.InvolvedObject.Kind,
		event.Namespace,
		event.Reason,
		event.Message,
		event.InvolvedObject.Kind,
		event.ObjectMeta.Name,
	)
}

func toMap(slice []string) map[string]bool {
	m := make(map[string]bool, len(slice))
	for _, v := range slice {
		m[v] = true
	}
	return m
}
