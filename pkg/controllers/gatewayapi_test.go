package controllers

import (
	"testing"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	gatewayv1beta1 "sigs.k8s.io/gateway-api/apis/v1beta1"

	cloudshellv1alpha1 "github.com/cloudtty/cloudtty/pkg/apis/cloudshell/v1alpha1"
)

func TestSetGatewayAPIRoutePath(t *testing.T) {
	cloudshell := &cloudshellv1alpha1.CloudShell{
		ObjectMeta: metav1.ObjectMeta{Name: "demo", Namespace: "team-a"},
		Spec: cloudshellv1alpha1.CloudShellSpec{
			PathPrefix: "/shared/",
			PathSuffix: "/terminal/",
		},
	}

	got := SetGatewayAPIRoutePath(cloudshell)
	want := "/shared/apis/v1alpha1/cloudshell/team-a/demo/terminal"
	if got != want {
		t.Fatalf("unexpected Gateway API route path: got %q, want %q", got, want)
	}
}

func TestDesiredGatewayAPIRoute(t *testing.T) {
	cloudshell := &cloudshellv1alpha1.CloudShell{
		ObjectMeta: metav1.ObjectMeta{Name: "demo", Namespace: "team-a", UID: "cloudshell-uid"},
	}
	route := desiredGatewayAPIRoute(cloudshell, "worker-1", "gateway-system", "shared-gateway", "https")

	if route.Namespace != "team-a" || route.Name != "cloudshell-demo" {
		t.Fatalf("unexpected route identity: %s/%s", route.Namespace, route.Name)
	}
	if got := string(route.Spec.ParentRefs[0].Name); got != "shared-gateway" {
		t.Fatalf("unexpected parent Gateway: %q", got)
	}
	if got := string(*route.Spec.ParentRefs[0].Namespace); got != "gateway-system" {
		t.Fatalf("unexpected parent Gateway namespace: %q", got)
	}
	if got := string(*route.Spec.ParentRefs[0].SectionName); got != "https" {
		t.Fatalf("unexpected parent listener: %q", got)
	}
	if got := *route.Spec.Rules[0].Matches[0].Path.Value; got != "/apis/v1alpha1/cloudshell/team-a/demo" {
		t.Fatalf("unexpected route match path: %q", got)
	}
	if got := string(*route.Spec.Rules[0].Filters[0].URLRewrite.Path.ReplacePrefixMatch); got != "/" {
		t.Fatalf("unexpected rewrite target: %q", got)
	}
	if got := string(route.Spec.Rules[0].BackendRefs[0].Name); got != "worker-1" {
		t.Fatalf("unexpected backend Service: %q", got)
	}
	if got := *route.Spec.Rules[0].BackendRefs[0].Port; got != gatewayv1beta1.PortNumber(7681) {
		t.Fatalf("unexpected backend port: %d", got)
	}
}

func TestGatewayRouteReadiness(t *testing.T) {
	cloudshell := &cloudshellv1alpha1.CloudShell{
		ObjectMeta: metav1.ObjectMeta{Name: "demo", Namespace: "team-a"},
	}
	route := desiredGatewayAPIRoute(cloudshell, "worker-1", "gateway-system", "shared-gateway", "https")
	route.Generation = 1
	route.Status.Parents = []gatewayv1beta1.RouteParentStatus{{
		ParentRef: route.Spec.ParentRefs[0],
		Conditions: []metav1.Condition{
			{Type: string(gatewayv1beta1.RouteConditionAccepted), Status: metav1.ConditionTrue, Reason: "Accepted", ObservedGeneration: 1},
			{Type: string(gatewayv1beta1.RouteConditionResolvedRefs), Status: metav1.ConditionTrue, Reason: "ResolvedRefs", ObservedGeneration: 1},
		},
	}}

	ready, condition, err := gatewayRouteReadiness(route, "team-a", "gateway-system", "shared-gateway", "https")
	if err != nil {
		t.Fatalf("unexpected readiness error: %v", err)
	}
	if !ready || condition.Status != metav1.ConditionTrue {
		t.Fatalf("expected route to be ready, got ready=%t condition=%+v", ready, condition)
	}

	route.Status.Parents[0].Conditions[0] = metav1.Condition{
		Type:               string(gatewayv1beta1.RouteConditionAccepted),
		Status:             metav1.ConditionFalse,
		Reason:             string(gatewayv1beta1.RouteReasonNotAllowedByListeners),
		Message:            "namespace is not allowed",
		ObservedGeneration: 1,
	}
	ready, condition, err = gatewayRouteReadiness(route, "team-a", "gateway-system", "shared-gateway", "https")
	if err != nil {
		t.Fatalf("unexpected rejected-route error: %v", err)
	}
	if ready || condition.Status != metav1.ConditionFalse || condition.Reason != string(gatewayv1beta1.RouteReasonNotAllowedByListeners) {
		t.Fatalf("expected route rejection, got ready=%t condition=%+v", ready, condition)
	}
}

func TestGatewayRouteReadinessWaitsForCurrentGeneration(t *testing.T) {
	cloudshell := &cloudshellv1alpha1.CloudShell{
		ObjectMeta: metav1.ObjectMeta{Name: "demo", Namespace: "team-a"},
	}
	route := desiredGatewayAPIRoute(cloudshell, "worker-1", "gateway-system", "shared-gateway", "https")
	route.Generation = 2
	route.Status.Parents = []gatewayv1beta1.RouteParentStatus{{
		ParentRef: route.Spec.ParentRefs[0],
		Conditions: []metav1.Condition{
			{Type: string(gatewayv1beta1.RouteConditionAccepted), Status: metav1.ConditionTrue, Reason: "Accepted", ObservedGeneration: 1},
			{Type: string(gatewayv1beta1.RouteConditionResolvedRefs), Status: metav1.ConditionTrue, Reason: "ResolvedRefs", ObservedGeneration: 1},
		},
	}}

	ready, condition, err := gatewayRouteReadiness(route, "team-a", "gateway-system", "shared-gateway", "https")
	if err != nil {
		t.Fatalf("unexpected stale-status error: %v", err)
	}
	if ready || condition.Status != metav1.ConditionUnknown {
		t.Fatalf("expected stale route status to remain pending, got ready=%t condition=%+v", ready, condition)
	}
}

func TestSetGatewayRouteConditionPreservesTransitionTime(t *testing.T) {
	cloudshell := &cloudshellv1alpha1.CloudShell{
		ObjectMeta: metav1.ObjectMeta{Name: "demo", Namespace: "team-a"},
		Status: cloudshellv1alpha1.CloudShellStatus{Conditions: []metav1.Condition{{
			Type:               cloudshellv1alpha1.GatewayRouteReadyCondition,
			Status:             metav1.ConditionUnknown,
			Reason:             "Pending",
			Message:            "waiting",
			LastTransitionTime: metav1.NewTime(time.Unix(100, 0)),
		}}},
	}

	setGatewayRouteCondition(cloudshell, metav1.Condition{
		Type:               cloudshellv1alpha1.GatewayRouteReadyCondition,
		Status:             metav1.ConditionUnknown,
		Reason:             "Pending",
		Message:            "waiting",
		LastTransitionTime: metav1.Now(),
	})
	if got := cloudshell.Status.Conditions[0].LastTransitionTime.Time; !got.Equal(time.Unix(100, 0)) {
		t.Fatalf("transition time changed for an unchanged condition: %v", got)
	}
}

func TestGatewayRouteIsPending(t *testing.T) {
	cloudshell := &cloudshellv1alpha1.CloudShell{}
	if !gatewayRouteIsPending(cloudshell) {
		t.Fatal("a CloudShell without a route condition should be pending")
	}

	cloudshell.Status.Conditions = []metav1.Condition{{
		Type:   cloudshellv1alpha1.GatewayRouteReadyCondition,
		Status: metav1.ConditionFalse,
	}}
	if gatewayRouteIsPending(cloudshell) {
		t.Fatal("a rejected Gateway route should not be polled continuously")
	}
}

func TestClearGatewayRouteStatus(t *testing.T) {
	cloudshell := &cloudshellv1alpha1.CloudShell{
		Status: cloudshellv1alpha1.CloudShellStatus{
			AccessURL: "/apis/v1alpha1/cloudshell/team-a/demo",
			Conditions: []metav1.Condition{{
				Type:   cloudshellv1alpha1.GatewayRouteReadyCondition,
				Status: metav1.ConditionTrue,
			}},
		},
	}

	clearGatewayRouteStatus(cloudshell)
	if cloudshell.Status.AccessURL != "" {
		t.Fatalf("expected access URL to be cleared, got %q", cloudshell.Status.AccessURL)
	}
	condition := cloudshell.Status.Conditions[0]
	if condition.Status != metav1.ConditionFalse || condition.Reason != "RouteRemoved" {
		t.Fatalf("expected removed route condition, got %+v", condition)
	}
}
