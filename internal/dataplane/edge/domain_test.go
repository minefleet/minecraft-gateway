package edge

import (
	"reflect"
	"sort"
	"testing"

	"minefleet.dev/minecraft-gateway/api/controller/v1alpha1"
	"minefleet.dev/minecraft-gateway/internal/route"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"
)

// --- helpers ---

func sortedCopy(in []string) []string {
	out := append([]string(nil), in...)
	sort.Strings(out)
	return out
}

func assertElementsMatch(t *testing.T, got, want []string) {
	t.Helper()
	if !reflect.DeepEqual(sortedCopy(got), sortedCopy(want)) {
		t.Fatalf("unexpected domains\n got:  %#v\n want: %#v", sortedCopy(got), sortedCopy(want))
	}
}

func joinRouteWithHostnames(hostnames ...string) route.Route {
	hs := make([]gatewayv1.Hostname, len(hostnames))
	for i, h := range hostnames {
		hs[i] = gatewayv1.Hostname(h)
	}
	return route.ForJoin(&v1alpha1.MinecraftJoinRoute{
		Spec: v1alpha1.MinecraftJoinRouteSpec{
			Hostnames: hs,
		},
	})
}

// --- Domains() tests ---

func TestDomains_EmptyBag(t *testing.T) {
	got := Domains(route.Bag{})
	assertElementsMatch(t, got, nil)
}

func TestDomains_SingleRoute(t *testing.T) {
	bag := route.Bag{
		Join: []route.Route{
			joinRouteWithHostnames("a.com", "b.com"),
		},
	}
	got := Domains(bag)
	assertElementsMatch(t, got, []string{"a.com", "b.com"})
}

func TestDomains_MultipleJoinRoutes_MergesAndDedupes(t *testing.T) {
	bag := route.Bag{
		Join: []route.Route{
			joinRouteWithHostnames("a.com", "b.com"),
			joinRouteWithHostnames("a.com", "c.com"),
		},
	}
	got := Domains(bag)
	assertElementsMatch(t, got, []string{"a.com", "b.com", "c.com"})
}

func TestDomains_WildcardAcrossRoutes_CoversSpecific(t *testing.T) {
	bag := route.Bag{
		Join: []route.Route{
			joinRouteWithHostnames("a.b.com"),
			joinRouteWithHostnames("*.b.com"),
		},
	}
	got := Domains(bag)
	assertElementsMatch(t, got, []string{"*.b.com"})
}

func TestDomains_WildcardAcrossRoutes_NotCoversParent(t *testing.T) {
	bag := route.Bag{
		Join: []route.Route{
			joinRouteWithHostnames("b.com"),
			joinRouteWithHostnames("*.b.com"),
		},
	}
	got := Domains(bag)
	assertElementsMatch(t, got, []string{"b.com", "*.b.com"})
}

func TestDomains_SkipsEmptyHostnames(t *testing.T) {
	bag := route.Bag{
		Join: []route.Route{
			joinRouteWithHostnames("", "a.com", ""),
		},
	}
	got := Domains(bag)
	assertElementsMatch(t, got, []string{"a.com"})
}
