package edge

import (
	"reflect"
	"sort"
	"testing"

	mcgatewayv1 "minefleet.dev/minecraft-gateway/api/v1"
	"minefleet.dev/minecraft-gateway/internal/route"
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

func rule(domain string) mcgatewayv1.MinecraftJoinFilterRule {
	return mcgatewayv1.MinecraftJoinFilterRule{
		MinecraftFilterRule: mcgatewayv1.MinecraftFilterRule{
			Domain: domain,
		},
	}
}

func setAny(domains ...string) mcgatewayv1.MinecraftJoinFilterRuleSet {
	rules := make([]mcgatewayv1.MinecraftJoinFilterRule, 0, len(domains))
	for _, d := range domains {
		rules = append(rules, rule(d))
	}
	return mcgatewayv1.MinecraftJoinFilterRuleSet{
		MinecraftFilterRuleSet: mcgatewayv1.MinecraftFilterRuleSet{
			Type: mcgatewayv1.MinecraftFilterRuleAny,
		},
		Rules: rules,
	}
}

func setAll(domains ...string) mcgatewayv1.MinecraftJoinFilterRuleSet {
	rules := make([]mcgatewayv1.MinecraftJoinFilterRule, 0, len(domains))
	for _, d := range domains {
		rules = append(rules, rule(d))
	}
	return mcgatewayv1.MinecraftJoinFilterRuleSet{
		MinecraftFilterRuleSet: mcgatewayv1.MinecraftFilterRuleSet{
			Type: mcgatewayv1.MinecraftFilterRuleAll,
		},
		Rules: rules,
	}
}

func setNone() mcgatewayv1.MinecraftJoinFilterRuleSet {
	return mcgatewayv1.MinecraftJoinFilterRuleSet{
		MinecraftFilterRuleSet: mcgatewayv1.MinecraftFilterRuleSet{
			Type: mcgatewayv1.MinecraftFilterRuleNone,
		},
	}
}

func joinRoute(sets ...mcgatewayv1.MinecraftJoinFilterRuleSet) mcgatewayv1.MinecraftJoinRoute {
	return mcgatewayv1.MinecraftJoinRoute{
		Spec: mcgatewayv1.MinecraftJoinRouteSpec{
			FilterRules: sets,
		},
	}
}

// --- tests ---

func TestSetDomains_None(t *testing.T) {
	got := setDomains(setNone())
	assertElementsMatch(t, got, nil)
}

func TestSetDomains_Any_DedupesAndSkipsEmpty(t *testing.T) {
	got := setDomains(setAny("a.com", "", "a.com", "b.com"))
	assertElementsMatch(t, got, []string{"a.com", "b.com"})
}

func TestSetDomains_All_FirstNonEmptyOnly(t *testing.T) {
	got := setDomains(setAll("", "a.com", "b.com"))
	// All: returns first non-empty domain only
	assertElementsMatch(t, got, []string{"a.com"})
}

func TestSetDomains_Any_WildcardCoversSpecific(t *testing.T) {
	got := setDomains(setAny("a.b.com", "*.b.com"))
	// *.b.com covers a.b.com, so only wildcard remains
	assertElementsMatch(t, got, []string{"*.b.com"})
}

func TestSetListDomains_DedupeAcrossSets(t *testing.T) {
	rules := []mcgatewayv1.MinecraftJoinFilterRuleSet{
		setAny("a.b.com"),
		setAny("*.b.com"),
	}
	got := setListDomains(rules)
	assertElementsMatch(t, got, []string{"*.b.com"})
}

func TestDomains_EmptyBag(t *testing.T) {
	got := Domains(route.Bag{})
	assertElementsMatch(t, got, nil)
}

func TestDomains_MultipleJoinRoutes_MergesAndDedupes(t *testing.T) {
	bag := route.Bag{
		Join: []mcgatewayv1.MinecraftJoinRoute{
			joinRoute(setAny("a.com", "b.com")),
			joinRoute(setAny("a.com", "c.com")),
		},
	}

	got := Domains(bag)
	assertElementsMatch(t, got, []string{"a.com", "b.com", "c.com"})
}

func TestDomains_WildcardAcrossRoutes_CoversSpecific(t *testing.T) {
	bag := route.Bag{
		Join: []mcgatewayv1.MinecraftJoinRoute{
			joinRoute(setAny("a.b.com")),
			joinRoute(setAny("*.b.com")),
		},
	}

	got := Domains(bag)
	assertElementsMatch(t, got, []string{"*.b.com"})
}

func TestDomains_AllBeatsAnyWithinASetOnly(t *testing.T) {
	// This test clarifies current semantics:
	// - A set of type All returns ONLY the first non-empty domain in that set.
	// - Domains() merges across routes/sets, then dedupes.
	bag := route.Bag{
		Join: []mcgatewayv1.MinecraftJoinRoute{
			joinRoute(
				setAny("a.com", "b.com"),
				setAll("x.com", "y.com"), // only x.com should appear from this set
			),
		},
	}

	got := Domains(bag)
	assertElementsMatch(t, got, []string{"a.com", "b.com", "x.com"})
}
