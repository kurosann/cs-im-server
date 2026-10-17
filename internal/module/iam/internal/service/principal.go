package service

const (
	PrincipalKindBusiness = "business"
	PrincipalKindRelay    = "relay"
)

func (p Principal) HasKind(kind string) bool {
	if p.Kind == "" {
		return kind == PrincipalKindBusiness
	}
	return p.Kind == kind
}
