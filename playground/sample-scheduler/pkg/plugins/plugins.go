package plugins

import (
	"context"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/klog"
	framework "k8s.io/kubernetes/pkg/scheduler/framework/v1alpha1"
	"k8s.io/kubernetes/pkg/scheduler/nodeinfo"
)

const Name = "sample-plugin"

type Args struct {
	KubeConfig string `json:"kubeconfig,omitempty"`
	Master     string `json:"master,omitempty"`
}

type Sample struct {
	args   *Args
	handle framework.FrameworkHandle
}

var (
	_ framework.FilterPlugin = &Sample{}
)

func (s *Sample) Name() string {
	return Name
}

func (s *Sample) Filter(ctx context.Context, state *framework.CycleState, pod *v1.Pod, nodeName *nodeinfo.NodeInfo) *framework.Status {
	klog.V(3).Infof("filter pod: %v, node: %v", pod.Name, nodeName)
	return framework.NewStatus(framework.Success, "")
}

//type PluginFactory = func(configuration *runtime.Unknown, f FrameworkHandle) (Plugin, error)
func New(configuration *runtime.Unknown, f framework.FrameworkHandle) (framework.Plugin, error) {
	args := &Args{}
	if err := framework.DecodeInto(configuration, args); err != nil {
		return nil, err
	}
	klog.V(3).Infof("get plugin config args: %+v", args)
	return &Sample{
		args:   args,
		handle: f,
	}, nil
}
