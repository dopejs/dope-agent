package opsreadiness

import (
	"encoding/json"
	"fmt"
	"strings"
)

var HostedRawCredentialMarkers = []string{
	"raw_secret",
	"access_token",
	"refresh_token",
	"oauth_code",
	"provider_token",
	"authorization:",
	"bearer ",
	"client_secret",
	"api_key=",
	"password=",
	"do_not_leak",
}

func ValidateHostedRedaction(label string, payload any) error {
	raw, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("%s redaction payload cannot be encoded: %w", label, err)
	}
	body := strings.ToLower(string(raw))
	for _, marker := range HostedRawCredentialMarkers {
		if strings.Contains(body, strings.ToLower(marker)) {
			return fmt.Errorf("%s contains raw credential material", label)
		}
	}
	return nil
}
