package sensitive

import "encoding/json"

type Sensitive string

func (s Sensitive) Plaintext() string {
	return string(s)
}

func (s Sensitive) String() string {
	if s == "" {
		return ""
	}
	return "[REDACTED]"
}

func (s Sensitive) MarshalJSON() ([]byte, error) {
	return json.Marshal(s.String())
}
