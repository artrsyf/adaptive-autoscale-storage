package distribution

type ModuloAlgorithm struct{}

func NewModuloAlgorithm() ModuloAlgorithm {
	return ModuloAlgorithm{}
}

func (ModuloAlgorithm) Name() string {
	return "modulo"
}

func (ModuloAlgorithm) Build(buildRequest BuildRequest) (Placement, error) {
	if err := validateNodes(buildRequest.Nodes); err != nil {
		return nil, err
	}
	nodes := append([]NodeID(nil), buildRequest.Nodes...)
	return &moduloPlacement{nodes: nodes}, nil
}

type moduloPlacement struct {
	nodes []NodeID
}

func (moduloPlacement *moduloPlacement) Owner(key string) NodeID {
	return moduloPlacement.nodes[hashValue(key)%uint64(len(moduloPlacement.nodes))]
}

func (*moduloPlacement) Metadata() PlacementMetadata {
	return PlacementMetadata{}
}
