package opsreadiness

import (
	"testing"
	"time"
)

func TestHostedProfileLayoutDefaultsAndProvisioningElapsed(t *testing.T) {
	profile := sampleHostedProfile()
	assertValid(t, ValidateHostedProfile(profile))
	assertValid(t, ValidateHostedProvisioningElapsed(59*time.Minute))

	profile.LiveConnectorMode = HostedLiveConnectorsLive
	assertInvalidContains(t, ValidateHostedProfile(profile), "live connector")
	assertInvalidContains(t, ValidateHostedProvisioningElapsed(61*time.Minute), "exceeds")
}
