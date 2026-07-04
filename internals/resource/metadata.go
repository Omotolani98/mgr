package resource

type Metadata struct {
	Name      string `json:"name" yaml:"name"`
	Namespace string `json:"namespace" yaml:"namespace"`
	Labels    string `json:"labels" yaml:"labels"`
}
