package servicediscovery

import (
	"context"
	"errors"
	"fmt"
	"sync"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/client-go/dynamic"
	"k8s.io/klog/v2"
)

const probeObjectName = "__kueue_crd_probe__"

// CRDStatus captures the last observed registration state for a controller CRD.
type CRDStatus struct {
	Enabled bool
	GVR     schema.GroupVersionResource
	Err     error
}

var statusCache = struct {
	lock        sync.RWMutex
	results     map[Controller]CRDStatus
	initialized bool
}{}

// IsCRDEnabled returns the cached registration state for the controller CRD.
// It never queries the API server directly.
func IsCRDEnabled(controller Controller) bool {
	status, found := GetCRDStatus(controller)
	return found && status.Enabled
}

// GetCRDStatus returns the cached CRD status and whether the cache has been
// initialized for the controller.
func GetCRDStatus(controller Controller) (CRDStatus, bool) {
	statusCache.lock.RLock()
	defer statusCache.lock.RUnlock()

	if !statusCache.initialized {
		return CRDStatus{}, false
	}
	status, found := statusCache.results[controller]
	return status, found
}

// EnumerateCRDs refreshes the cached CRD registration results using the dynamic client.
func EnumerateCRDs(dynamicClient dynamic.Interface, namespace string) (map[Controller]CRDStatus, error) {
	return EnumerateCRDsForControllers(dynamicClient, namespace, allControllers)
}

// EnumerateCRDsForControllers refreshes the cached CRD registration results for the requested controllers.
func EnumerateCRDsForControllers(dynamicClient dynamic.Interface, namespace string, controllers []Controller) (map[Controller]CRDStatus, error) {
	if dynamicClient == nil {
		return nil, errors.New("dynamic client is required")
	}
	if namespace == "" {
		return nil, errors.New("namespace is required")
	}

	selectedControllers := dedupeControllers(controllers)
	results := make(map[Controller]CRDStatus, len(selectedControllers))
	var errs []error

	for _, controller := range selectedControllers {
		meta, found := registry[controller]
		if !found {
			errs = append(errs, fmt.Errorf("missing registry metadata for controller %q", controller))
			continue
		}

		status := CRDStatus{GVR: meta.gvr}
		err := probeResource(context.Background(), dynamicClient, meta, namespace)
		switch {
		case err == nil:
			status.Enabled = true
		case apierrors.IsNotFound(err):
			status.Enabled = notFoundMeansEnabled(err)
		default:
			status.Err = err
			errs = append(errs, fmt.Errorf("enumerating %s: %w", controller, err))
		}

		if status.Enabled {
			klog.V(2).InfoS("CRD discovered", "controller", controller, "gvr", meta.gvr)
		} else {
			klog.V(2).InfoS("CRD not found", "controller", controller, "gvr", meta.gvr)
		}

		results[controller] = status
	}

	statusCache.lock.Lock()
	statusCache.results = cloneResults(results)
	statusCache.initialized = true
	statusCache.lock.Unlock()

	return cloneResults(results), utilerrors.NewAggregate(errs)
}

func cloneResults(in map[Controller]CRDStatus) map[Controller]CRDStatus {
	out := make(map[Controller]CRDStatus, len(in))
	for key, value := range in {
		out[key] = value
	}
	return out
}

func dedupeControllers(controllers []Controller) []Controller {
	if len(controllers) == 0 {
		return nil
	}

	seen := make(map[Controller]struct{}, len(controllers))
	ret := make([]Controller, 0, len(controllers))
	for _, controller := range controllers {
		if _, found := seen[controller]; found {
			continue
		}
		seen[controller] = struct{}{}
		ret = append(ret, controller)
	}
	return ret
}

func probeResource(ctx context.Context, dynamicClient dynamic.Interface, meta crdMetadata, namespace string) error {
	switch meta.probeVerb {
	case probeVerbGet:
		if meta.namespaced {
			_, err := dynamicClient.Resource(meta.gvr).Namespace(namespace).Get(ctx, probeObjectName, metav1.GetOptions{})
			return err
		}
		_, err := dynamicClient.Resource(meta.gvr).Get(ctx, probeObjectName, metav1.GetOptions{})
		return err
	default:
		return fmt.Errorf("unsupported probe verb %q for %s", meta.probeVerb, meta.gvr.String())
	}
}

func notFoundMeansEnabled(err error) bool {
	statusErr, ok := err.(*apierrors.StatusError)
	if !ok {
		return false
	}
	details := statusErr.Status().Details
	return details != nil && details.Name == probeObjectName
}

func resetCache() {
	statusCache.lock.Lock()
	defer statusCache.lock.Unlock()

	statusCache.results = nil
	statusCache.initialized = false
}
