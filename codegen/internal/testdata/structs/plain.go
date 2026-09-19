package fixtures

// User is a person.
// Second comment line.
type User struct {
	ID         int      `json:"id"`
	Name       string   `json:"name"`
	Nickname   string   `json:"nickname,omitempty"`
	Avatar     *string  `json:"avatar"`
	Secret     string   `json:"-"`
	Tags       []string `json:"tags"`
	Data       []byte   `json:"data"`
	NoTag      bool
	unexported string `json:"unexported"`
}

// unexportedType must be skipped entirely.
type unexportedType struct {
	Field string `json:"field"`
}
