package servicediscovery

import "k8s.io/apimachinery/pkg/runtime/schema"

type crdMetadata struct {
	gvr        schema.GroupVersionResource
	probeVerb  probeVerb
	namespaced bool
}

type probeVerb string

const (
	probeVerbGet probeVerb = "get"
)

var registry = map[Controller]crdMetadata{
	AppWrapper: {
		gvr:        schema.GroupVersionResource{Group: "workload.codeflare.dev", Version: "v1beta2", Resource: "appwrappers"},
		probeVerb:  probeVerbGet,
		namespaced: true,
	},
	JobSet: {
		gvr:        schema.GroupVersionResource{Group: "jobset.x-k8s.io", Version: "v1alpha2", Resource: "jobsets"},
		probeVerb:  probeVerbGet,
		namespaced: true,
	},
	LeaderWorkerSet: {
		gvr:        schema.GroupVersionResource{Group: "leaderworkerset.x-k8s.io", Version: "v1", Resource: "leaderworkersets"},
		probeVerb:  probeVerbGet,
		namespaced: true,
	},
	MPIJob: {
		gvr:        schema.GroupVersionResource{Group: "kubeflow.org", Version: "v2beta1", Resource: "mpijobs"},
		probeVerb:  probeVerbGet,
		namespaced: true,
	},
	JAXJob: {
		gvr:        schema.GroupVersionResource{Group: "kubeflow.org", Version: "v1", Resource: "jaxjobs"},
		probeVerb:  probeVerbGet,
		namespaced: true,
	},
	PaddleJob: {
		gvr:        schema.GroupVersionResource{Group: "kubeflow.org", Version: "v1", Resource: "paddlejobs"},
		probeVerb:  probeVerbGet,
		namespaced: true,
	},
	PyTorchJob: {
		gvr:        schema.GroupVersionResource{Group: "kubeflow.org", Version: "v1", Resource: "pytorchjobs"},
		probeVerb:  probeVerbGet,
		namespaced: true,
	},
	TFJob: {
		gvr:        schema.GroupVersionResource{Group: "kubeflow.org", Version: "v1", Resource: "tfjobs"},
		probeVerb:  probeVerbGet,
		namespaced: true,
	},
	XGBoostJob: {
		gvr:        schema.GroupVersionResource{Group: "kubeflow.org", Version: "v1", Resource: "xgboostjobs"},
		probeVerb:  probeVerbGet,
		namespaced: true,
	},
	TrainJob: {
		gvr:        schema.GroupVersionResource{Group: "trainer.kubeflow.org", Version: "v1alpha1", Resource: "trainjobs"},
		probeVerb:  probeVerbGet,
		namespaced: true,
	},
	RayCluster: {
		gvr:        schema.GroupVersionResource{Group: "ray.io", Version: "v1", Resource: "rayclusters"},
		probeVerb:  probeVerbGet,
		namespaced: true,
	},
	RayJob: {
		gvr:        schema.GroupVersionResource{Group: "ray.io", Version: "v1", Resource: "rayjobs"},
		probeVerb:  probeVerbGet,
		namespaced: true,
	},
	RayService: {
		gvr:        schema.GroupVersionResource{Group: "ray.io", Version: "v1", Resource: "rayservices"},
		probeVerb:  probeVerbGet,
		namespaced: true,
	},
	ProvisioningRequest: {
		gvr:        schema.GroupVersionResource{Group: "autoscaling.x-k8s.io", Version: "v1", Resource: "provisioningrequests"},
		probeVerb:  probeVerbGet,
		namespaced: true,
	},
	ClusterProfile: {
		gvr:        schema.GroupVersionResource{Group: "multicluster.x-k8s.io", Version: "v1alpha1", Resource: "clusterprofiles"},
		probeVerb:  probeVerbGet,
		namespaced: false,
	},
}

var frameworkControllers = map[string]Controller{
	"workload.codeflare.dev/appwrapper":        AppWrapper,
	"jobset.x-k8s.io/jobset":                   JobSet,
	"leaderworkerset.x-k8s.io/leaderworkerset": LeaderWorkerSet,
	"kubeflow.org/mpijob":                      MPIJob,
	"kubeflow.org/jaxjob":                      JAXJob,
	"kubeflow.org/paddlejob":                   PaddleJob,
	"kubeflow.org/pytorchjob":                  PyTorchJob,
	"kubeflow.org/tfjob":                       TFJob,
	"kubeflow.org/xgboostjob":                  XGBoostJob,
	"trainer.kubeflow.org/trainjob":            TrainJob,
	"ray.io/raycluster":                        RayCluster,
	"ray.io/rayjob":                            RayJob,
	"ray.io/rayservice":                        RayService,
}

func GVRForController(controller Controller) (schema.GroupVersionResource, bool) {
	meta, found := registry[controller]
	if !found {
		return schema.GroupVersionResource{}, false
	}
	return meta.gvr, true
}

func ControllerForFramework(framework string) (Controller, bool) {
	controller, found := frameworkControllers[framework]
	return controller, found
}

func ControllersForFrameworks(frameworks []string) []Controller {
	if len(frameworks) == 0 {
		return nil
	}

	seen := make(map[Controller]struct{}, len(frameworks))
	ret := make([]Controller, 0, len(frameworks))
	for _, framework := range frameworks {
		controller, found := frameworkControllers[framework]
		if !found {
			continue
		}
		if _, alreadyAdded := seen[controller]; alreadyAdded {
			continue
		}
		seen[controller] = struct{}{}
		ret = append(ret, controller)
	}
	return ret
}
