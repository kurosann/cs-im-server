package service

const (
	PrincipalKindBusiness = "business"
	PrincipalKindRelay    = "relay"
)

func (p Principal) HasScope(scope string) bool {
	if scope == "" {
		return false
	}
	for _, value := range p.Scopes {
		if value == scope || value == "*" {
			return true
		}
	}
	return false
}

func (p Principal) HasKind(kind string) bool {
	if p.Kind == "" {
		return kind == PrincipalKindBusiness
	}
	return p.Kind == kind
}
