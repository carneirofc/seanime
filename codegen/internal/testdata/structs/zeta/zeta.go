package zetapkg

// Zeta is in a package that sorts last, to exercise package ordering.
type Zeta struct {
	ID    int    `json:"id"`
	Label string `json:"label"`
}
