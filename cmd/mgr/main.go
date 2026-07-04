package main

import (
	"github.com/Omotolani98/mgr/internals/manager"
	"github.com/Omotolani98/mgr/internals/resource"
)

func main() {
	mgr := manager.Manager{}

	pod := &resource.Pod{
		Kind: "Pod",
		Metadata: resource.Metadata{
			Name: "my-pod",
		},
		Containers: []resource.Container{
			{
				Name:  "my-container",
				Image: "nginx:latest",
				Port:  80,
			},
		},
	}

	service := &resource.Service{
		Kind: "Service",
		Metadata: resource.Metadata{
			Name: "my-service",
		},
	}

	mgr.Add(pod)
	mgr.Add(service)
}
