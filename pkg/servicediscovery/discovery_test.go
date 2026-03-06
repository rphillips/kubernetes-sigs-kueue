package servicediscovery

import (
	"errors"
	"testing"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic/fake"
	kubetesting "k8s.io/client-go/testing"
)

const testNamespace = "kueue-system"

func TestControllerRegistryCoverage(t *testing.T) {
	t.Helper()

	seen := make(map[string]struct{}, len(allControllers))
	for _, controller := range allControllers {
		name := controller.String()
		if name == "" {
			t.Fatalf("controller %d has empty string form", controller)
		}
		if _, found := seen[name]; found {
			t.Fatalf("duplicate controller string %q", name)
		}
		seen[name] = struct{}{}

		meta, found := registry[controller]
		if !found {
			t.Fatalf("controller %q missing registry metadata", controller)
		}
		if meta.gvr.Empty() {
			t.Fatalf("controller %q has empty GVR", controller)
		}
	}
}

func TestControllersForFrameworks(t *testing.T) {
	got := ControllersForFrameworks([]string{
		"batch/job",
		"ray.io/rayjob",
		"kubeflow.org/mpijob",
		"ray.io/rayjob",
		"unknown/framework",
	})

	want := []Controller{RayJob, MPIJob}
	if len(got) != len(want) {
		t.Fatalf("ControllersForFrameworks() returned %d controllers, want %d", len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("ControllersForFrameworks()[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestIsCRDEnabledUsesCachedResults(t *testing.T) {
	resetCache()

	if IsCRDEnabled(RayJob) {
		t.Fatalf("expected false before cache initialization")
	}

	client := newDynamicClientWithStatuses(t, map[Controller]error{
		RayJob: nil,
	})

	if _, err := EnumerateCRDs(client, testNamespace); err != nil {
		t.Fatalf("EnumerateCRDs() error = %v", err)
	}

	if !IsCRDEnabled(RayJob) {
		t.Fatalf("expected RayJob to be enabled from cache")
	}
	if IsCRDEnabled(MPIJob) {
		t.Fatalf("expected MPIJob to remain disabled in cache")
	}
}

func TestEnumerateCRDsRefreshesCache(t *testing.T) {
	resetCache()

	client := newDynamicClientWithStatuses(t, map[Controller]error{
		RayJob: nil,
	})

	if _, err := EnumerateCRDs(client, testNamespace); err != nil {
		t.Fatalf("EnumerateCRDs() initial error = %v", err)
	}
	if !IsCRDEnabled(RayJob) {
		t.Fatalf("expected RayJob enabled after first enumeration")
	}

	client = newDynamicClientWithStatuses(t, map[Controller]error{
		RayJob: apierrors.NewNotFound(schema.GroupResource{
			Group:    registry[RayJob].gvr.Group,
			Resource: registry[RayJob].gvr.Resource,
		}, ""),
	})

	if _, err := EnumerateCRDs(client, testNamespace); err != nil {
		t.Fatalf("EnumerateCRDs() refresh error = %v", err)
	}
	if IsCRDEnabled(RayJob) {
		t.Fatalf("expected RayJob disabled after cache refresh")
	}
}

func TestEnumerateCRDsReturnsStatusForAllControllers(t *testing.T) {
	resetCache()

	client := newDynamicClientWithStatuses(t, map[Controller]error{
		RayJob:              nil,
		ProvisioningRequest: nil,
	})

	got, err := EnumerateCRDs(client, testNamespace)
	if err != nil {
		t.Fatalf("EnumerateCRDs() error = %v", err)
	}

	if len(got) != len(allControllers) {
		t.Fatalf("EnumerateCRDs() returned %d results, want %d", len(got), len(allControllers))
	}
	if !got[RayJob].Enabled {
		t.Fatalf("expected RayJob enabled")
	}
	if !got[ProvisioningRequest].Enabled {
		t.Fatalf("expected ProvisioningRequest enabled")
	}
	if got[MPIJob].Enabled {
		t.Fatalf("expected MPIJob disabled")
	}
	if got[MPIJob].Err != nil {
		t.Fatalf("expected MPIJob disabled without error, got %v", got[MPIJob].Err)
	}
}

func TestEnumerateCRDsAggregatesUnexpectedErrors(t *testing.T) {
	resetCache()

	expectedErr := errors.New("boom")
	client := newDynamicClientWithStatuses(t, map[Controller]error{
		RayJob: expectedErr,
	})

	got, err := EnumerateCRDs(client, testNamespace)
	if err == nil {
		t.Fatalf("expected aggregate error")
	}
	if got[RayJob].Enabled {
		t.Fatalf("expected RayJob disabled on error")
	}
	if !errors.Is(got[RayJob].Err, expectedErr) {
		t.Fatalf("expected cached error %v, got %v", expectedErr, got[RayJob].Err)
	}
}

func TestEnumerateCRDsForControllersOnlyCachesRequestedControllers(t *testing.T) {
	resetCache()

	client := newDynamicClientWithStatuses(t, map[Controller]error{
		RayJob: nil,
	})

	got, err := EnumerateCRDsForControllers(client, testNamespace, []Controller{RayJob, RayJob})
	if err != nil {
		t.Fatalf("EnumerateCRDsForControllers() error = %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("EnumerateCRDsForControllers() returned %d results, want 1", len(got))
	}
	if !got[RayJob].Enabled {
		t.Fatalf("expected RayJob enabled")
	}
	if !IsCRDEnabled(RayJob) {
		t.Fatalf("expected RayJob enabled in cache")
	}
	if _, found := GetCRDStatus(MPIJob); found {
		t.Fatalf("expected MPIJob to be absent from cache when not enumerated")
	}
}

func newDynamicClientWithStatuses(t *testing.T, statuses map[Controller]error) *fake.FakeDynamicClient {
	t.Helper()

	client := fake.NewSimpleDynamicClient(runtime.NewScheme())
	client.PrependReactor("get", "*", func(action kubetesting.Action) (bool, runtime.Object, error) {
		getAction, ok := action.(kubetesting.GetAction)
		if !ok {
			t.Fatalf("unexpected action type %T", action)
		}

		gvr := getAction.GetResource()
		controller, found := controllerForGVR(gvr)
		if !found {
			t.Fatalf("unexpected GVR %s", gvr.String())
		}
		if registry[controller].namespaced && getAction.GetNamespace() != testNamespace {
			t.Fatalf("controller %q probe namespace = %q, want %q", controller, getAction.GetNamespace(), testNamespace)
		}
		if !registry[controller].namespaced && getAction.GetNamespace() != "" {
			t.Fatalf("controller %q expected cluster-scoped probe, got namespace %q", controller, getAction.GetNamespace())
		}

		err, found := statuses[controller]
		if !found {
			return true, nil, apierrors.NewNotFound(schema.GroupResource{Group: gvr.Group, Resource: gvr.Resource}, "")
		}
		if err != nil {
			return true, nil, err
		}

		return true, nil, apierrors.NewNotFound(schema.GroupResource{Group: gvr.Group, Resource: gvr.Resource}, probeObjectName)
	})

	return client
}

func controllerForGVR(gvr schema.GroupVersionResource) (Controller, bool) {
	for controller, meta := range registry {
		if meta.gvr == gvr {
			return controller, true
		}
	}
	return 0, false
}

func TestEnumerateCRDsRejectsNilClient(t *testing.T) {
	resetCache()

	if _, err := EnumerateCRDs(nil, testNamespace); err == nil {
		t.Fatalf("expected nil client error")
	}
	if _, err := EnumerateCRDs(fake.NewSimpleDynamicClient(runtime.NewScheme()), ""); err == nil {
		t.Fatalf("expected empty namespace error")
	}
}

func TestEnumerateCRDsTreatsNoMatchAsUnexpectedError(t *testing.T) {
	resetCache()

	noMatch := &meta.NoKindMatchError{GroupKind: schema.GroupKind{Group: "ray.io", Kind: "RayJob"}}
	client := newDynamicClientWithStatuses(t, map[Controller]error{
		RayJob: noMatch,
	})

	got, err := EnumerateCRDs(client, testNamespace)
	if err == nil {
		t.Fatalf("expected error for no match")
	}
	if !errors.Is(got[RayJob].Err, noMatch) {
		t.Fatalf("expected no-match error to be preserved, got %v", got[RayJob].Err)
	}
}

func TestEnumerateCRDsReturnsCopyOfCache(t *testing.T) {
	resetCache()

	client := newDynamicClientWithStatuses(t, map[Controller]error{
		RayJob: nil,
	})

	got, err := EnumerateCRDs(client, testNamespace)
	if err != nil {
		t.Fatalf("EnumerateCRDs() error = %v", err)
	}

	status := got[RayJob]
	status.Enabled = false
	got[RayJob] = status

	if !IsCRDEnabled(RayJob) {
		t.Fatalf("expected cache to remain unchanged after caller mutates result map")
	}
}

func TestEnumerateCRDsPreservesGVRMetadata(t *testing.T) {
	resetCache()

	client := newDynamicClientWithStatuses(t, nil)

	got, err := EnumerateCRDs(client, testNamespace)
	if err != nil {
		t.Fatalf("EnumerateCRDs() error = %v", err)
	}

	for _, controller := range allControllers {
		if got[controller].GVR != registry[controller].gvr {
			t.Fatalf("controller %q returned GVR %v, want %v", controller, got[controller].GVR, registry[controller].gvr)
		}
	}
}

func TestEnumerateCRDsUsesGetProbe(t *testing.T) {
	resetCache()

	client := fake.NewSimpleDynamicClient(runtime.NewScheme())
	client.PrependReactor("get", "*", func(action kubetesting.Action) (bool, runtime.Object, error) {
		getAction := action.(kubetesting.GetAction)
		if getAction.GetName() != probeObjectName {
			t.Fatalf("unexpected probe object name %q", getAction.GetName())
		}
		controller, found := controllerForGVR(getAction.GetResource())
		if !found {
			t.Fatalf("unexpected GVR %s", getAction.GetResource().String())
		}
		if registry[controller].namespaced && getAction.GetNamespace() != testNamespace {
			t.Fatalf("controller %q probe namespace = %q, want %q", controller, getAction.GetNamespace(), testNamespace)
		}
		if !registry[controller].namespaced && getAction.GetNamespace() != "" {
			t.Fatalf("controller %q expected cluster-scoped probe, got namespace %q", controller, getAction.GetNamespace())
		}
		return true, nil, apierrors.NewNotFound(schema.GroupResource{
			Group:    getAction.GetResource().Group,
			Resource: getAction.GetResource().Resource,
		}, "")
	})

	if _, err := EnumerateCRDs(client, testNamespace); err != nil {
		t.Fatalf("EnumerateCRDs() error = %v", err)
	}
}

func TestEnumerateCRDsStoresDisabledEntriesInCache(t *testing.T) {
	resetCache()

	client := newDynamicClientWithStatuses(t, nil)
	if _, err := EnumerateCRDs(client, testNamespace); err != nil {
		t.Fatalf("EnumerateCRDs() error = %v", err)
	}

	if IsCRDEnabled(ClusterProfile) {
		t.Fatalf("expected disabled CRD to be cached as false")
	}
}

func TestEnumerateCRDsResultIncludesErrorsWithoutPanickingOnMutation(t *testing.T) {
	resetCache()

	expectedErr := apierrors.NewInternalError(errors.New("internal"))
	client := newDynamicClientWithStatuses(t, map[Controller]error{
		ClusterProfile: expectedErr,
	})

	got, err := EnumerateCRDs(client, testNamespace)
	if err == nil {
		t.Fatalf("expected aggregate error")
	}
	if got[ClusterProfile].Err == nil {
		t.Fatalf("expected ClusterProfile status error")
	}
}
