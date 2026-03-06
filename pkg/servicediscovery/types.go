package servicediscovery

import "fmt"

// Controller identifies an external controller backed by a CRD.
type Controller int

const (
	AppWrapper Controller = iota
	JobSet
	LeaderWorkerSet
	MPIJob
	JAXJob
	PaddleJob
	PyTorchJob
	TFJob
	XGBoostJob
	TrainJob
	RayCluster
	RayJob
	RayService
	ProvisioningRequest
	ClusterProfile
)

var allControllers = []Controller{
	AppWrapper,
	JobSet,
	LeaderWorkerSet,
	MPIJob,
	JAXJob,
	PaddleJob,
	PyTorchJob,
	TFJob,
	XGBoostJob,
	TrainJob,
	RayCluster,
	RayJob,
	RayService,
	ProvisioningRequest,
	ClusterProfile,
}

func AllControllers() []Controller {
	ret := make([]Controller, len(allControllers))
	copy(ret, allControllers)
	return ret
}

func (c Controller) String() string {
	switch c {
	case AppWrapper:
		return "AppWrapper"
	case JobSet:
		return "JobSet"
	case LeaderWorkerSet:
		return "LeaderWorkerSet"
	case MPIJob:
		return "MPIJob"
	case JAXJob:
		return "JAXJob"
	case PaddleJob:
		return "PaddleJob"
	case PyTorchJob:
		return "PyTorchJob"
	case TFJob:
		return "TFJob"
	case XGBoostJob:
		return "XGBoostJob"
	case TrainJob:
		return "TrainJob"
	case RayCluster:
		return "RayCluster"
	case RayJob:
		return "RayJob"
	case RayService:
		return "RayService"
	case ProvisioningRequest:
		return "ProvisioningRequest"
	case ClusterProfile:
		return "ClusterProfile"
	default:
		return fmt.Sprintf("Controller(%d)", c)
	}
}
