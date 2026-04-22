package conformance

import (
	"context"
	"flag"
	"fmt"
	"os"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"
	"sigs.k8s.io/gateway-api/conformance"
	"sigs.k8s.io/gateway-api/conformance/tests"
	"sigs.k8s.io/gateway-api/conformance/utils/flags"
	"sigs.k8s.io/gateway-api/conformance/utils/suite"
	"sigs.k8s.io/gateway-api/conformance/utils/tlog"
	"sigs.k8s.io/gateway-api/pkg/features"
)

const (
	gatewayClassName = "minefleet"
	controllerName   = "minefleet.dev/gateway-controller"
)

func TestMain(m *testing.M) {
	cfg, err := config.GetConfig()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error loading Kubernetes config: %v\n", err)
		os.Exit(1)
	}

	c, err := client.New(cfg, client.Options{})
	if err != nil {
		fmt.Fprintf(os.Stderr, "error creating Kubernetes client: %v\n", err)
		os.Exit(1)
	}
	if err := gatewayv1.Install(c.Scheme()); err != nil {
		fmt.Fprintf(os.Stderr, "error registering gateway API scheme: %v\n", err)
		os.Exit(1)
	}

	ctx := context.Background()
	gwClass := &gatewayv1.GatewayClass{
		ObjectMeta: metav1.ObjectMeta{
			Name: gatewayClassName,
		},
		Spec: gatewayv1.GatewayClassSpec{
			ControllerName: gatewayv1.GatewayController(controllerName),
		},
	}
	if err := c.Create(ctx, gwClass); client.IgnoreAlreadyExists(err) != nil {
		fmt.Fprintf(os.Stderr, "error creating GatewayClass: %v\n", err)
		os.Exit(1)
	}

	code := m.Run()

	_ = c.Delete(ctx, gwClass)
	os.Exit(code)
}

func conformanceOpts(t *testing.T) suite.ConformanceOptions {
	opts := conformance.DefaultOptions(t)
	supportedFeatures := sets.New(
		features.SupportGateway,
		features.SupportGatewayAddressEmpty,
		features.SupportReferenceGrant,
		features.SupportGatewayInfrastructurePropagation,
	)
	opts.SupportedFeatures = supportedFeatures
	return opts
}

func TestGatewayAPIConformance(t *testing.T) {
	flag.Parse()

	if flags.RunTest != nil && *flags.RunTest != "" {
		tlog.Logf(t, "Running Conformance test %s with %s GatewayClass\n cleanup: %t\n debug: %t",
			*flags.RunTest, *flags.GatewayClassName, *flags.CleanupBaseResources, *flags.ShowDebug)
	} else {
		tlog.Logf(t, "Running Conformance tests with %s GatewayClass\n cleanup: %t\n debug: %t",
			*flags.GatewayClassName, *flags.CleanupBaseResources, *flags.ShowDebug)
	}

	opts := conformanceOpts(t)
	opts.RunTest = *flags.RunTest
	opts.GatewayClassName = gatewayClassName

	// If focusing on a single test, clear the skip list to ensure it runs.
	if opts.RunTest != "" {
		opts.SkipTests = nil
	}
	cSuite, err := suite.NewConformanceTestSuite(opts)
	if err != nil {
		t.Fatalf("Error creating conformance test suite: %v", err)
	}
	cSuite.Setup(t, tests.ConformanceTests)
	if err := cSuite.Run(t, tests.ConformanceTests); err != nil {
		t.Fatalf("Error running conformance tests: %v", err)
	}
}
