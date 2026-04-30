package opsreadiness

func LoadHostedEvidenceFixture[T any](path string) (T, error) {
	return LoadJSONFixture[T](path)
}
