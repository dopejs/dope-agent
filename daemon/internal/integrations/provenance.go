package integrations

func updateProvenance(resource Resource, input UpdateReadinessInput) Resource {
	if input.SecretResolution != "" {
		resource.Provenance.SecretResolution = input.SecretResolution
		resource.Provenance.SecretMaterialPresent = input.SecretResolution == "resolved"
	}
	if resource.Provenance.EnvironmentScope == "" {
		resource.Provenance.EnvironmentScope = resource.EnvironmentScope
	}
	if resource.Provenance.BackedBy == "" {
		resource.Provenance.BackedBy = string(resource.BackendBinding.BackendKind)
	}
	return resource
}
