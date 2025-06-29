package controller

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/go-logr/logr"
	appsv1 "k8s.io/api/apps/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	gamev1 "game-scaler-operator/api/v1" // Переконайтеся, що шлях правильний
)

// EventAPIResponse represents the structure of the response from your game event API
type EventAPIResponse struct {
	EventType          string `json:"eventType"`
	StartTime          string `json:"startTime"` // ISO 8601 format
	EndTime            string `json:"endTime"`   // ISO 8601 format
	TargetMicroservice string `json:"targetMicroservice"`
	// Add other fields from your API response if needed
}

// GameEventScaleRuleReconciler reconciles a GameEventScaleRule object
type GameEventScaleRuleReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=game.game.yourdomain.com,resources=gameeventscalerules,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=game.game.yourdomain.com,resources=gameeventscalerules/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=game.game.yourdomain.com,resources=gameeventscalerules/finalizers,verbs=update
//+kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch;update;patch

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
func (r *GameEventScaleRuleReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	// 1. Fetch the GameEventScaleRule instance
	gameEventScaleRule := &gamev1.GameEventScaleRule{}
	if err := r.Get(ctx, req.NamespacedName, gameEventScaleRule); err != nil {
		if apierrors.IsNotFound(err) {
			logger.Info("GameEventScaleRule resource not found. Ignoring since object must be deleted")
			return ctrl.Result{}, nil
		}
		logger.Error(err, "Failed to get GameEventScaleRule")
		return ctrl.Result{}, err
	}

	// 2. Determine polling interval for the external event API
	pollingDuration, err := time.ParseDuration(gameEventScaleRule.Spec.PollingInterval)
	if err != nil {
		logger.Error(err, "Invalid PollingInterval format", "interval", gameEventScaleRule.Spec.PollingInterval)
		return ctrl.Result{RequeueAfter: time.Minute}, nil // Requeue with a default interval on error
	}

	// 3. Check if it's time to poll the external event API
	if gameEventScaleRule.Status.LastEventCheckTime != nil {
		if time.Since(gameEventScaleRule.Status.LastEventCheckTime.Time) < pollingDuration {
			// Not time to check yet, requeue after the remaining duration
			requeueAfter := pollingDuration - time.Since(gameEventScaleRule.Status.LastEventCheckTime.Time)
			logger.V(1).Info("Next event check in", "duration", requeueAfter)
			return ctrl.Result{RequeueAfter: requeueAfter}, nil
		}
	}

	// 4. Poll the external game event API
	logger.Info("Polling external game event API", "URL", gameEventScaleRule.Spec.EventEndpointURL)
	events, err := r.pollGameEventAPI(gameEventScaleRule.Spec.EventEndpointURL, logger)
	if err != nil {
		logger.Error(err, "Failed to poll game event API")
		// Update status to reflect error, but continue reconciliation to try again later
		gameEventScaleRule.Status.LastEventCheckTime = &metav1.Time{Time: time.Now()} // Update last check time even on error
		if updateErr := r.Status().Update(ctx, gameEventScaleRule); updateErr != nil {
			logger.Error(updateErr, "Failed to update GameEventScaleRule status after API polling error")
		}
		return ctrl.Result{RequeueAfter: pollingDuration}, err // Requeue after polling interval
	}

	gameEventScaleRule.Status.LastEventCheckTime = &metav1.Time{Time: time.Now()}

	// 5. Process events and scale deployments
	newActiveScales := []gamev1.ActiveScaleStatus{}
	requeueSoon := false // Flag to requeue if a scaling operation is imminent

	for _, rule := range gameEventScaleRule.Spec.Rules {
		foundEvent := false
		for _, event := range events {
			if event.EventType == rule.EventType {
				foundEvent = true
				logger.V(1).Info("Found matching event for rule", "eventType", rule.EventType, "microservice", rule.TargetMicroservice)

				startTime, err := time.Parse(time.RFC3339, event.StartTime)
				if err != nil {
					logger.Error(err, "Failed to parse event StartTime", "startTime", event.StartTime)
					continue
				}
				endTime, err := time.Parse(time.RFC3339, event.EndTime)
				if err != nil {
					logger.Error(err, "Failed to parse event EndTime", "endTime", event.EndTime)
					continue
				}

				// Calculate scale up time
				scaleUpTime := startTime.Add(-time.Duration(rule.PreScaleMinutes) * time.Minute)
				// Calculate scale down time
				scaleDownTime := endTime.Add(time.Duration(rule.PostScaleMinutes) * time.Minute)

				now := time.Now()

				deployment := &appsv1.Deployment{}
				deployNN := types.NamespacedName{Name: rule.TargetMicroservice, Namespace: req.Namespace} // Assume same namespace
				if err := r.Get(ctx, deployNN, deployment); err != nil {
					if apierrors.IsNotFound(err) {
						logger.Info("Deployment not found for scaling rule", "deployment", rule.TargetMicroservice)
						continue
					}
					logger.Error(err, "Failed to get Deployment", "deployment", rule.TargetMicroservice)
					continue
				}

				currentReplicas := *deployment.Spec.Replicas

				// Logic for scaling UP
				if now.After(scaleUpTime) && now.Before(endTime) {
					if currentReplicas != rule.DesiredReplicas {
						logger.Info("Scaling up Deployment", "deployment", rule.TargetMicroservice, "from", currentReplicas, "to", rule.DesiredReplicas, "for event", rule.EventType)
						if err := r.scaleDeployment(ctx, deployNN, rule.DesiredReplicas); err != nil {
							logger.Error(err, "Failed to scale up Deployment", "deployment", rule.TargetMicroservice)
							continue
						}
						// Add to active scales
						newActiveScales = append(newActiveScales, gamev1.ActiveScaleStatus{
							EventType:          rule.EventType,
							TargetMicroservice: rule.TargetMicroservice,
							ScaledToReplicas:   rule.DesiredReplicas,
							ScaleTriggerTime:   &metav1.Time{Time: now},
							EventEndTime:       &metav1.Time{Time: endTime},
							Status:             "Active",
						})
					} else {
						logger.V(1).Info("Deployment already at desired replicas for event", "deployment", rule.TargetMicroservice, "replicas", currentReplicas)
						// Keep current active status if already scaled
						newActiveScales = append(newActiveScales, gamev1.ActiveScaleStatus{
							EventType:          rule.EventType,
							TargetMicroservice: rule.TargetMicroservice,
							ScaledToReplicas:   rule.DesiredReplicas,
							ScaleTriggerTime:   &metav1.Time{Time: now}, // Update time to keep it "fresh"
							EventEndTime:       &metav1.Time{Time: endTime},
							Status:             "Active",
						})
					}
				} else if now.After(scaleDownTime) && currentReplicas != rule.DefaultReplicas { // Logic for scaling DOWN after event
					logger.Info("Scaling down Deployment after event", "deployment", rule.TargetMicroservice, "from", currentReplicas, "to", rule.DefaultReplicas, "for event", rule.EventType)
					if err := r.scaleDeployment(ctx, deployNN, rule.DefaultReplicas); err != nil {
						logger.Error(err, "Failed to scale down Deployment", "deployment", rule.TargetMicroservice)
						continue
					}
					// Mark as completed or remove from active scales
				} else {
					logger.V(1).Info("No scaling action needed for event rule at this time", "eventType", rule.EventType)
					// If the event hasn't started yet but is imminent, requeue soon
					if now.Before(scaleUpTime) && scaleUpTime.Sub(now) < pollingDuration {
						requeueSoon = true
					}
					// If event is active and not finished, keep it in active scales
					if now.After(scaleUpTime) && now.Before(scaleDownTime) && currentReplicas == rule.DesiredReplicas {
						newActiveScales = append(newActiveScales, gamev1.ActiveScaleStatus{
							EventType:          rule.EventType,
							TargetMicroservice: rule.TargetMicroservice,
							ScaledToReplicas:   rule.DesiredReplicas,
							ScaleTriggerTime:   &metav1.Time{Time: now},
							EventEndTime:       &metav1.Time{Time: endTime},
							Status:             "Active",
						})
					}
				}
				break // Found event for this rule, move to next rule
			}
		}
		if !foundEvent {
			// If no current event matches this rule, ensure it's at default replicas
			deployment := &appsv1.Deployment{}
			deployNN := types.NamespacedName{Name: rule.TargetMicroservice, Namespace: req.Namespace}
			if err := r.Get(ctx, deployNN, deployment); err == nil {
				currentReplicas := *deployment.Spec.Replicas
				if currentReplicas != rule.DefaultReplicas {
					logger.Info("No active event, scaling Deployment to default replicas", "deployment", rule.TargetMicroservice, "from", currentReplicas, "to", rule.DefaultReplicas)
					if err := r.scaleDeployment(ctx, deployNN, rule.DefaultReplicas); err != nil {
						logger.Error(err, "Failed to scale to default replicas", "deployment", rule.TargetMicroservice)
					}
				}
			}
		}
	}

	gameEventScaleRule.Status.ActiveScales = newActiveScales

	// 6. Update the status of the GameEventScaleRule
	if err := r.Status().Update(ctx, gameEventScaleRule); err != nil {
		logger.Error(err, "Failed to update GameEventScaleRule status")
		return ctrl.Result{}, err
	}

	// Requeue after polling interval or sooner if an event is imminent
	if requeueSoon {
		logger.Info("Requeuing soon for imminent event")
		return ctrl.Result{RequeueAfter: 30 * time.Second}, nil // Requeue faster if an event is about to start
	}
	return ctrl.Result{RequeueAfter: pollingDuration}, nil
}

// pollGameEventAPI fetches event data from the external API endpoint
func (r *GameEventScaleRuleReconciler) pollGameEventAPI(url string, logger logr.Logger) ([]EventAPIResponse, error) {
	resp, err := http.Get(url)
	if err != nil {
		return nil, fmt.Errorf("failed to make HTTP request to %s: %w", url, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("received non-OK status from %s: %s", url, resp.Status)
	}

	var events []EventAPIResponse
	if err := json.NewDecoder(resp.Body).Decode(&events); err != nil {
		return nil, fmt.Errorf("failed to decode API response from %s: %w", url, err)
	}
	return events, nil
}

// scaleDeployment updates the replicas of a given Deployment
func (r *GameEventScaleRuleReconciler) scaleDeployment(ctx context.Context, name types.NamespacedName, replicas int32) error {
	deployment := &appsv1.Deployment{}
	if err := r.Get(ctx, name, deployment); err != nil {
		return fmt.Errorf("failed to get Deployment %s: %w", name.Name, err)
	}

	patch := client.MergeFrom(deployment.DeepCopy())
	deployment.Spec.Replicas = &replicas

	if err := r.Patch(ctx, deployment, patch); err != nil {
		return fmt.Errorf("failed to patch Deployment %s to %d replicas: %w", name.Name, replicas, err)
	}
	return nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *GameEventScaleRuleReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&gamev1.GameEventScaleRule{}).
		Owns(&appsv1.Deployment{}). // Watch Deployments owned by this controller (optional, but good practice)
		Complete(r)
}
