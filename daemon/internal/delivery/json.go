package delivery

import "encoding/json"

func marshal(value any) ([]byte, error) {
	return json.Marshal(value)
}

func unmarshalDocument(document []byte, target any) error {
	return json.Unmarshal(document, target)
}
