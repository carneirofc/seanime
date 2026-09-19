package fixtures

// Status is an enum-like alias.
type Status string

const (
	StatusActive   Status = "active"
	StatusInactive Status = "inactive"
	statusHidden   Status = "hidden"
)

// SelfAlias must be skipped: alias name equals type name.
type SelfAlias SelfAlias

// Lookup is a map alias.
type Lookup map[string]User

// List is a slice alias.
type List []User

// Count is a numeric alias with no declared values.
type Count int
