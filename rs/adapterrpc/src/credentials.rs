//! Per-call scoped credential resolution (Roadmap 37 secret path). Mirrors
//! `credentials.go`.

use serde_json::value::RawValue;

use crate::client::{CredentialResolver, ResolverError};

/// Returns scoped, short-lived credential material for one integration. At wiring time it
/// is backed by the Roadmap 37 secret path; it MUST return fresh material per call and MUST
/// NOT be cached by the adapter.
pub type IntegrationCredentialFetcher =
    Box<dyn Fn(&str) -> Result<Option<Box<RawValue>>, ResolverError> + Send + Sync>;

/// Build a per-call [`CredentialResolver`] that derives the integration id from the
/// resource envelope and fetches fresh credential material for each call. Because it
/// fetches per call and threads nothing through the shared adapter process, concurrent
/// calls for different tenants cannot share credential state (FR-006, FR-015).
pub fn scoped_resolver(fetch: Option<IntegrationCredentialFetcher>) -> CredentialResolver {
    Box::new(move |_domain, resource| {
        let id = integration_id_from_resource(resource);
        let (Some(fetch), Some(id)) = (fetch.as_ref(), id) else {
            return Ok(None);
        };
        fetch(&id)
    })
}

fn integration_id_from_resource(resource: Option<&RawValue>) -> Option<String> {
    let raw = resource?;
    #[derive(serde::Deserialize)]
    #[serde(rename_all = "camelCase")]
    struct Resource {
        #[serde(default)]
        integration_id: String,
    }
    let r: Resource = serde_json::from_str(raw.get()).ok()?;
    if r.integration_id.is_empty() {
        None
    } else {
        Some(r.integration_id)
    }
}
