package alphapkg

// Alpha is in a package that sorts first, to exercise package ordering.
type Alpha struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}
