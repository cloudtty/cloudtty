package controllers

import (
	"context"
	"fmt"
	"reflect"
	"strings"
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	ctrlutil "sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	gatewayv1beta1 "sigs.k8s.io/gateway-api/apis/v1beta1"

	cloudshellv1alpha1 "github.com/cloudtty/cloudtty/pkg/apis/cloudshell/v1alpha1"
	"github.com/cloudtty/cloudtty/pkg/constants"
)

const (
	// gatewayAPIRouteRequeuePeriod gives the Gateway controller time to publish
	// HTTPRoute status. The CloudShell controller uses a workqueue, so polling is
	// the least invasive way to observe a route controller that is not owned by
	// CloudTTY.
	gatewayAPIRouteRequeuePeriod = 2 * time.Second

	gatewayAPIRouteNamePrefix = "cloudshell-"
	gatewayAPIRouteLabel      = "cloudtty.io/cloudshell"
)

// ensureGatewayAPIRoute creates or updates the HTTPRoute owned by a CloudShell
// and then reads the latest route status. The Gateway itself is never changed.
func (c *Controller) ensureGatewayAPIRoute(ctx context.Context, cloudshell *cloudshellv1alpha1.CloudShell, workerName string) (bool, metav1.Condition, error) {
	if !c.gatewayAPIConfigured() {
		return false, metav1.Condition{}, fmt.Errorf("GatewayAPI exposure requires --gateway-api-gateway-name and --gateway-api-gateway-namespace")
	}

	desired := desiredGatewayAPIRoute(cloudshell, workerName, c.gatewayAPIGatewayNamespace, c.gatewayAPIGatewayName, c.gatewayAPISectionName)
	route := &gatewayv1beta1.HTTPRoute{}
	err := c.Get(ctx, client.ObjectKeyFromObject(desired), route)
	if apierrors.IsNotFound(err) {
		if err := ctrlutil.SetControllerReference(cloudshell, desired, c.Scheme); err != nil {
			return false, metav1.Condition{}, err
		}
		if err := c.Create(ctx, desired); err != nil {
			return false, metav1.Condition{}, err
		}
		return false, gatewayRouteCondition(cloudshell, metav1.ConditionUnknown, "Pending", "HTTPRoute has been created and is waiting for the Gateway controller"), nil
	}
	if err != nil {
		return false, metav1.Condition{}, err
	}

	if !metav1.IsControlledBy(route, cloudshell) {
		return false, metav1.Condition{}, fmt.Errorf("HTTPRoute %s/%s is not owned by CloudShell %s/%s", route.Namespace, route.Name, cloudshell.Namespace, cloudshell.Name)
	}

	oldSpec := route.Spec.DeepCopy()
	oldOwnerReferences := append([]metav1.OwnerReference(nil), route.OwnerReferences...)
	route.Spec = desired.Spec
	if err := ctrlutil.SetControllerReference(cloudshell, route, c.Scheme); err != nil {
		return false, metav1.Condition{}, err
	}
	if !reflect.DeepEqual(oldSpec, &route.Spec) || !reflect.DeepEqual(oldOwnerReferences, route.OwnerReferences) {
		if err := c.Update(ctx, route); err != nil {
			return false, metav1.Condition{}, err
		}
	}

	// A spec update can temporarily leave the old status in the object returned
	// by Update. Read the object again so readiness is based on the API server's
	// current status rather than on a stale in-memory copy.
	if err := c.Get(ctx, client.ObjectKeyFromObject(route), route); err != nil {
		return false, metav1.Condition{}, err
	}

	return gatewayRouteReadiness(route, cloudshell.Namespace, c.gatewayAPIGatewayNamespace, c.gatewayAPIGatewayName, c.gatewayAPISectionName)
}

// deleteGatewayAPIRoute removes only the route owned by the target CloudShell.
// This is intentionally explicit because cleanup:false leaves the CloudShell
// object behind, and ownerReference garbage collection therefore cannot help.
func (c *Controller) deleteGatewayAPIRoute(ctx context.Context, cloudshell *cloudshellv1alpha1.CloudShell) error {
	route := &gatewayv1beta1.HTTPRoute{}
	key := client.ObjectKey{Namespace: cloudshell.Namespace, Name: gatewayAPIRouteName(cloudshell)}
	if err := c.Get(ctx, key, route); err != nil {
		if apierrors.IsNotFound(err) {
			return nil
		}
		return err
	}

	if !metav1.IsControlledBy(route, cloudshell) {
		return fmt.Errorf("refusing to delete HTTPRoute %s/%s because it is not owned by CloudShell %s/%s", route.Namespace, route.Name, cloudshell.Namespace, cloudshell.Name)
	}
	return c.Delete(ctx, route)
}

func (c *Controller) gatewayAPIConfigured() bool {
	return c.gatewayAPIGatewayName != "" && c.gatewayAPIGatewayNamespace != ""
}

func desiredGatewayAPIRoute(cloudshell *cloudshellv1alpha1.CloudShell, workerName, gatewayNamespace, gatewayName, sectionName string) *gatewayv1beta1.HTTPRoute {
	routePath := SetGatewayAPIRoutePath(cloudshell)
	pathType := gatewayv1beta1.PathMatchPathPrefix
	replacePrefix := "/"
	port := gatewayv1beta1.PortNumber(7681)
	gatewayNamespaceRef := gatewayv1beta1.Namespace(gatewayNamespace)

	parentRef := gatewayv1beta1.ParentReference{
		Name:      gatewayv1beta1.ObjectName(gatewayName),
		Namespace: &gatewayNamespaceRef,
	}
	if sectionName != "" {
		section := gatewayv1beta1.SectionName(sectionName)
		parentRef.SectionName = &section
	}

	return &gatewayv1beta1.HTTPRoute{
		ObjectMeta: metav1.ObjectMeta{
			Name:      gatewayAPIRouteName(cloudshell),
			Namespace: cloudshell.Namespace,
			Labels: map[string]string{
				gatewayAPIRouteLabel: cloudshell.Name,
			},
		},
		Spec: gatewayv1beta1.HTTPRouteSpec{
			CommonRouteSpec: gatewayv1beta1.CommonRouteSpec{
				ParentRefs: []gatewayv1beta1.ParentReference{parentRef},
			},
			Rules: []gatewayv1beta1.HTTPRouteRule{{
				Matches: []gatewayv1beta1.HTTPRouteMatch{{
					Path: &gatewayv1beta1.HTTPPathMatch{
						Type:  &pathType,
						Value: &routePath,
					},
				}},
				Filters: []gatewayv1beta1.HTTPRouteFilter{{
					Type: gatewayv1beta1.HTTPRouteFilterURLRewrite,
					URLRewrite: &gatewayv1beta1.HTTPURLRewriteFilter{
						Path: &gatewayv1beta1.HTTPPathModifier{
							Type:               gatewayv1beta1.PrefixMatchHTTPPathModifier,
							ReplacePrefixMatch: &replacePrefix,
						},
					},
				}},
				BackendRefs: []gatewayv1beta1.HTTPBackendRef{{
					BackendRef: gatewayv1beta1.BackendRef{
						BackendObjectReference: gatewayv1beta1.BackendObjectReference{
							Name: gatewayv1beta1.ObjectName(workerName),
							Port: &port,
						},
					},
				}},
			}},
		},
	}
}

func gatewayAPIRouteName(cloudshell *cloudshellv1alpha1.CloudShell) string {
	return gatewayAPIRouteNamePrefix + cloudshell.Name
}

// SetGatewayAPIRoutePath returns a path that is unique across namespaces while
// keeping the existing CloudShell pathPrefix/pathSuffix contract.
func SetGatewayAPIRoutePath(cloudshell *cloudshellv1alpha1.CloudShell) string {
	pathPrefix := strings.TrimSuffix(cloudshell.Spec.PathPrefix, "/")
	pathPrefix += constants.DefaultPathPrefix
	path := fmt.Sprintf("%s/%s/%s/%s", pathPrefix, cloudshell.Namespace, cloudshell.Name, strings.Trim(cloudshell.Spec.PathSuffix, "/"))
	return strings.TrimSuffix(path, "/")
}

func gatewayRouteReadiness(route *gatewayv1beta1.HTTPRoute, routeNamespace, gatewayNamespace, gatewayName, sectionName string) (bool, metav1.Condition, error) {
	for _, parent := range route.Status.Parents {
		if !gatewayParentMatches(parent.ParentRef, routeNamespace, gatewayNamespace, gatewayName, sectionName) {
			continue
		}

		accepted := routeParentCondition(parent.Conditions, gatewayv1beta1.RouteConditionAccepted)
		resolvedRefs := routeParentCondition(parent.Conditions, gatewayv1beta1.RouteConditionResolvedRefs)
		if accepted == nil || resolvedRefs == nil {
			return false, pendingGatewayRouteCondition(route, "Gateway controller has not reported Accepted and ResolvedRefs"), nil
		}
		if accepted.ObservedGeneration < route.Generation || resolvedRefs.ObservedGeneration < route.Generation {
			return false, pendingGatewayRouteCondition(route, "Gateway controller has not observed the current HTTPRoute generation"), nil
		}
		if accepted.Status != metav1.ConditionTrue {
			return false, gatewayRouteConditionFromRoute(route, accepted), nil
		}
		if resolvedRefs.Status != metav1.ConditionTrue {
			return false, gatewayRouteConditionFromRoute(route, resolvedRefs), nil
		}
		return true, gatewayRouteConditionFromRoute(route, accepted), nil
	}

	return false, pendingGatewayRouteCondition(route, "HTTPRoute is waiting for the configured Gateway"), nil
}

func gatewayParentMatches(parent gatewayv1beta1.ParentReference, routeNamespace, gatewayNamespace, gatewayName, sectionName string) bool {
	if string(parent.Name) != gatewayName {
		return false
	}
	if parent.Namespace == nil {
		if routeNamespace != gatewayNamespace {
			return false
		}
	} else if string(*parent.Namespace) != gatewayNamespace {
		return false
	}
	if sectionName != "" && (parent.SectionName == nil || string(*parent.SectionName) != sectionName) {
		return false
	}
	return true
}

func routeParentCondition(conditions []metav1.Condition, conditionType gatewayv1beta1.RouteConditionType) *metav1.Condition {
	for i := range conditions {
		if conditions[i].Type == string(conditionType) {
			return &conditions[i]
		}
	}
	return nil
}

func pendingGatewayRouteCondition(route *gatewayv1beta1.HTTPRoute, message string) metav1.Condition {
	condition := gatewayRouteConditionFromRoute(route, &metav1.Condition{
		Type:    string(gatewayv1beta1.RouteConditionAccepted),
		Status:  metav1.ConditionUnknown,
		Reason:  "Pending",
		Message: message,
	})
	if condition.LastTransitionTime.IsZero() {
		condition.LastTransitionTime = metav1.Now()
	}
	return condition
}

func gatewayRouteConditionFromRoute(route *gatewayv1beta1.HTTPRoute, condition *metav1.Condition) metav1.Condition {
	return metav1.Condition{
		Type:               cloudshellv1alpha1.GatewayRouteReadyCondition,
		Status:             condition.Status,
		ObservedGeneration: route.Generation,
		LastTransitionTime: condition.LastTransitionTime,
		Reason:             condition.Reason,
		Message:            condition.Message,
	}
}

func gatewayRouteCondition(cloudshell *cloudshellv1alpha1.CloudShell, status metav1.ConditionStatus, reason, message string) metav1.Condition {
	return metav1.Condition{
		Type:               cloudshellv1alpha1.GatewayRouteReadyCondition,
		Status:             status,
		ObservedGeneration: cloudshell.Generation,
		LastTransitionTime: metav1.Now(),
		Reason:             reason,
		Message:            message,
	}
}

func setGatewayRouteCondition(cloudshell *cloudshellv1alpha1.CloudShell, condition metav1.Condition) {
	if condition.LastTransitionTime.IsZero() {
		condition.LastTransitionTime = metav1.Now()
	}
	for i := range cloudshell.Status.Conditions {
		old := &cloudshell.Status.Conditions[i]
		if old.Type != condition.Type {
			continue
		}
		if old.Status == condition.Status && old.Reason == condition.Reason && old.Message == condition.Message {
			condition.LastTransitionTime = old.LastTransitionTime
		}
		cloudshell.Status.Conditions[i] = condition
		return
	}
	cloudshell.Status.Conditions = append(cloudshell.Status.Conditions, condition)
}

func gatewayRouteIsPending(cloudshell *cloudshellv1alpha1.CloudShell) bool {
	for _, condition := range cloudshell.Status.Conditions {
		if condition.Type == cloudshellv1alpha1.GatewayRouteReadyCondition {
			return condition.Status == metav1.ConditionUnknown
		}
	}
	return true
}

func clearGatewayRouteStatus(cloudshell *cloudshellv1alpha1.CloudShell) {
	cloudshell.Status.AccessURL = ""
	setGatewayRouteCondition(cloudshell, metav1.Condition{
		Type:    cloudshellv1alpha1.GatewayRouteReadyCondition,
		Status:  metav1.ConditionFalse,
		Reason:  "RouteRemoved",
		Message: "HTTPRoute was removed because the CloudShell is no longer active",
	})
}
