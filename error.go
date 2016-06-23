package jpatch

// Error contains detailed information about what went wrong
type Error struct {
	Message string
	Details string
	Origin  error
}

func (p Error) Error() string {
	return p.Message
}
