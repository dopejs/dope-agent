//! Port of `daemon/internal/evaluation/product_validation.go`: tenant
//! scoping, page-limit bounds, and discovery policy validation.

use crate::error::EvaluationError;
use crate::types::DiscoveryPolicy;

/// Go `DefaultProductPageLimit` / `MaxProductPageLimit`.
pub const DEFAULT_PRODUCT_PAGE_LIMIT: i64 = 50;
pub const MAX_PRODUCT_PAGE_LIMIT: i64 = 200;

/// Go `ValidateTenantScopedProductRequest`.
pub fn validate_tenant_scoped_product_request(tenant_id: &str) -> Result<(), EvaluationError> {
    if tenant_id.trim().is_empty() {
        return Err(EvaluationError::ProductTenantRequired);
    }
    Ok(())
}

/// Go `NormalizeProductLimit`.
#[must_use]
pub fn normalize_product_limit(limit: i64) -> i64 {
    if limit <= 0 {
        return DEFAULT_PRODUCT_PAGE_LIMIT;
    }
    if limit > MAX_PRODUCT_PAGE_LIMIT {
        return MAX_PRODUCT_PAGE_LIMIT;
    }
    limit
}

/// Go `ValidateDiscoveryPolicy`.
pub fn validate_discovery_policy(policy: &DiscoveryPolicy) -> Result<(), EvaluationError> {
    validate_tenant_scoped_product_request(&policy.tenant_id)?;
    if policy.max_inspected_records <= 0
        || policy.max_emitted_candidates <= 0
        || policy.cost_budget <= 0
    {
        return Err(EvaluationError::ProductBoundsInvalid);
    }
    if !crate::campaign::is_zero_time(policy.window_start)
        && !crate::campaign::is_zero_time(policy.window_end)
        && policy.window_start >= policy.window_end
    {
        return Err(EvaluationError::ProductBoundsInvalid);
    }
    Ok(())
}
