package fixtures

// Parent has an inline anonymous struct field.
type Parent struct {
	Name   string `json:"name"`
	Nested struct {
		Enabled bool  `json:"enabled"`
		Numbers []int `json:"numbers"`
	} `json:"nested"`
}
