package resource

type Service struct {
	Kind string `json:"kind" yaml:"kind"`
	Metadata
}

func (s *Service) GetName() string {
	return s.Name
}

func (s *Service) GetKind() string {
	return s.Kind
}
