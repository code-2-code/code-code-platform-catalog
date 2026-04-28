package modelcatalogsources

// Source describes a model catalog capability that can be registered.
type Source interface {
	CapabilityRef() CapabilityRef
}
