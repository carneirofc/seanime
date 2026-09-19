package fixtures

type Base struct {
	CreatedAt string `json:"createdAt"`
}

// Derived embeds Base.
type Derived struct {
	Base
	Own string `json:"own"`
}
