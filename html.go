package gox

// HTML represents trusted HTML received through a GOX transport. Functions
// returning HTML remain compiler-rendered declarations; socket handlers may
// also use this type to receive an HTML argument without degrading it to text.
type HTML string

func (value HTML) String() string { return string(value) }
func (value HTML) Bytes() []byte  { return []byte(value) }
