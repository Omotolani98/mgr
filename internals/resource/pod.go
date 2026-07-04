package resource

type Pod struct {
	Kind string `json:"kind" yaml:"kind"`
	Metadata
	Containers []Container `json:"containers" yaml:"containers"`
	Restarts   int
}

type Container struct {
	Name  string `json:"name" yaml:"name"`
	Image string `json:"image" yaml:"image"`
	Port  int    `json:"port" yaml:"port"`
}

func (p *Pod) GetName() string {
	return p.Name
}

func (p *Pod) GetKind() string {
	return p.Kind
}
