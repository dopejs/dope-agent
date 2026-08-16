//! Request middleware: environment scope, bearer-token auth, tenant
//! resolution, and the by-id tenant ownership guard.
//!
//! Ports of the Go helpers in daemon/internal/api/server.go:
//! - `withEnvironment` -> [`with_environment`]
//! - `protected()`     -> [`protected`]
//! - `authenticateRequest` / `authTokenAuthority` -> private fns here
//! - `withByIDTenantGuard` / `guardResourceForTenant` -> [`guard_resource_for_tenant`]
//!   + [`ByIDTenantGuardLayer`]

use std::convert::Infallible;
use std::future::Future;
use std::pin::Pin;
use std::task::{Context, Poll};

use axum::extract::{Request, State};
use axum::http::header;
use axum::middleware::Next;
use axum::response::{IntoResponse, Response};
use tower::Service;

use dope_identity::auth::{AccessToken, AuthError, TokenStatus};
use dope_identity::tenantctx;
use dope_identity::{LifecycleStatus, TenantContext as ResolvedTenantContext, TokenAuthority};

use crate::error::ApiError;
use crate::state::AppState;

/// Environment scope string attached to every request (Go
/// `events.WithEnvironmentScope(ctx, env)` carried through the context).
#[derive(Debug, Clone)]
pub struct EnvironmentScope(pub String);

/// Authenticated access token attached by [`protected`] (Go
/// `withAuthenticatedToken`).
#[derive(Debug, Clone)]
pub struct AuthenticatedToken(pub AccessToken);

/// Resolved tenant context attached by [`protected`] (Go `withTenantContext`).
#[derive(Debug, Clone)]
pub struct TenantContext(pub ResolvedTenantContext);

/// Reads the environment scope from the request extensions, if present.
#[must_use]
pub fn environment_scope(req: &Request) -> Option<&str> {
    req.extensions().get::<EnvironmentScope>().map(|s| s.0.as_str())
}

/// Canonical environment string from the config (Go `effectiveEnvironment`).
#[must_use]
pub fn environment_scope_from_config(config: &dope_config::Config) -> String {
    match config.environment {
        dope_config::Environment::Prod => "prod".to_string(),
        dope_config::Environment::Test => "test".to_string(),
    }
}

/// `withEnvironment` equivalent: injects the daemon environment scope into the
/// request extensions for unauthenticated routes (pairing entry points).
#[allow(clippy::unused_async)]
pub async fn with_environment(State(state): State<AppState>, mut req: Request, next: Next) -> Response {
    req.extensions_mut()
        .insert(EnvironmentScope(environment_scope_from_config(&state.config)));
    next.run(req).await
}

/// Bearer-token authentication + tenant resolution, mirroring the Go
/// `protected()` middleware.
///
/// 1. Attaches the environment scope.
/// 2. Refuses tenant-owned requests while the tenant backfill migration is in
///    progress (503 + stable `tenant_migration_in_progress`).
/// 3. When an auth manager is configured: authenticates the `Authorization:
///    Bearer <secret>` token, persists it (store), resolves the tenant context
///    via the identity manager (when configured) using the
///    `X-Dope-Tenant-ID` header, and attaches [`AuthenticatedToken`] +
///    [`TenantContext`] extensions.
///
/// When no auth manager is configured the request passes through
/// unauthenticated (matching the Go nil-auth behavior).
#[allow(clippy::unused_async)]
pub async fn protected(
    State(state): State<AppState>,
    mut req: Request,
    next: Next,
) -> Result<Response, ApiError> {
    req.extensions_mut()
        .insert(EnvironmentScope(environment_scope_from_config(&state.config)));

    // Roadmap 35 (finding #4): the protected middleware refuses tenant-owned
    // requests while backfills are running so clients can backoff coherently.
    if let Some(status) = &state.tenant_migration_status {
        if status.in_progress() {
            return Err(ApiError::TenantMigrationInProgress);
        }
    }

    let tenant_header = req
        .headers()
        .get("x-dope-tenant-id")
        .and_then(|v| v.to_str().ok())
        .map(str::trim)
        .unwrap_or_default()
        .to_string();

    let Some(auth) = &state.auth else {
        // No auth manager configured: pass through (Go nil-auth behavior).
        return Ok(next.run(req).await);
    };

    let token = authenticate_request(auth, &req)?;

    // Go: persistAccessToken(r.Context(), store, token). The store owns the
    // access-token table; failures surface as 500.
    if let Err(e) = state.store.lock().upsert_access_token(&token) {
        return Err(ApiError::from_store(e));
    }

    req.extensions_mut().insert(AuthenticatedToken(token.clone()));

    if let Some(identity) = &state.identity {
        // Go: deps.Identity.Resolve(ctx, authTokenAuthority(token), tenantID).
        let resolved = identity
            .resolve(&auth_token_authority(&token), &tenant_header)
            .map_err(|err| match err {
                dope_identity::IdentityError::TenantAccessDenied => {
                    // Go: recordTenantAccessDenied + writeTenantDenial (403).
                    // The tenant_resolution_denied audit write and the full
                    // stable Denial body (error/errorCode/requestId) remain
                    // deferred; the 403 itself is ported.
                    ApiError::Forbidden("tenant access denied".to_string())
                }
                other => ApiError::internal(other),
            })?;
        req.extensions_mut().insert(TenantContext(resolved));
    }

    Ok(next.run(req).await)
}

/// `authenticateRequest` equivalent: extracts and validates the bearer token.
/// The auth-manager-nil case is handled by the caller (`protected` only calls
/// this when a manager is configured); an empty header maps to
/// `ErrAuthRequired` like the Go `if deps.Auth != nil && !ok` branch.
fn authenticate_request(
    auth: &dope_identity::auth::Manager,
    req: &Request,
) -> Result<AccessToken, ApiError> {
    let header = req
        .headers()
        .get(header::AUTHORIZATION)
        .and_then(|v| v.to_str().ok())
        .map(str::trim)
        .unwrap_or_default();
    if header.is_empty() {
        return Err(ApiError::Unauthorized(AuthError::AuthRequired.to_string()));
    }
    let secret = header
        .strip_prefix("Bearer ")
        .map(str::trim)
        .filter(|s| !s.is_empty())
        .ok_or_else(|| ApiError::Unauthorized(AuthError::TokenInvalid.to_string()))?;
    let token = auth
        .authenticate(secret)
        .map_err(|err| {
            // Go: token-expired requests additionally record a
            // tenant.access_denied audit event (reason_code=token_expired)
            // when the token identity is known. The Rust auth manager returns
            // no token alongside the error, so that audit path remains deferred.
            ApiError::Unauthorized(err.to_string())
        })?;
    Ok(token)
}

/// `authTokenAuthority` equivalent: projects an access token onto the identity
/// layer's token authority (Go defaults an empty status to Active).
#[must_use]
pub fn auth_token_authority(token: &AccessToken) -> TokenAuthority {
    let status = match token.status {
        TokenStatus::Active => LifecycleStatus::Active,
        TokenStatus::Revoked => LifecycleStatus::Revoked,
        TokenStatus::Expired => LifecycleStatus::Expired,
        TokenStatus::Rotated => LifecycleStatus::Rotated,
    };
    TokenAuthority {
        token_id: token.token_id.clone(),
        principal_id: token.principal_id.clone(),
        default_tenant_id: token.default_tenant_id.clone(),
        status,
        expires_at: token.expires_at,
    }
}

/// Core tenant-ownership check (Go `guardResourceForTenant`).
///
/// Returns `Ok(())` when the request may proceed: no tenant context, row
/// absent (the downstream handler decides), row owned by the caller, or a
/// legacy pre-backfill row (`tenant_id IS NULL`). Returns `Err(NotFound)` after
/// emitting an audit event when the row belongs to a different tenant (the
/// cross-tenant access is never leaked as such — 404). Any store failure
/// surfaces as `Err(Internal)`.
///
/// `surface` is the canonical `api:<METHOD route>` label used in audit
/// emissions; `table`/`pk_column` must be trusted compile-time constants.
pub async fn guard_resource_for_tenant(
    state: &AppState,
    tenant: Option<&TenantContext>,
    surface: &str,
    table: &str,
    pk_column: &str,
    pk_value: &str,
    resource_kind: &str,
) -> Result<(), ApiError> {
    let Some(tc) = tenant else {
        return Ok(());
    };
    if tc.0.tenant_id.is_empty() {
        return Ok(());
    }
    let owner = state
        .store
        .lock()
        .lookup_row_tenant(table, pk_column, pk_value)
        .map_err(ApiError::from_store)?;
    match owner {
        // Pre-backfill rows (NULL tenant) stay visible to every tenant.
        None => Ok(()),
        Some(o) if o.is_empty() || o == tc.0.tenant_id => Ok(()),
        Some(_) => {
            emit_tenant_breach(state, &tc.0, surface, resource_kind);
            Err(ApiError::NotFound("not found".to_string()))
        }
    }
}

/// Publishes the cross-tenant denial audit event. Safe no-op when no emitter
/// is configured. The emitter reads the acting tenant from the task-local
/// carrier, so the resolved tenant context is installed around the emit.
fn emit_tenant_breach(
    state: &AppState,
    tenant: &ResolvedTenantContext,
    surface: &str,
    resource_kind: &str,
) {
    let Some(emitter) = &state.audit_emitter else {
        return;
    };
    let _ = tenantctx::with_context(tenant.clone(), || emitter.emit(surface, resource_kind));
}

/// Canonical surface label for audit emissions (Go `surfaceFromRequest`):
/// `api:<METHOD route>`.
#[must_use]
pub fn surface_from_request(req: &Request) -> String {
    format!("api:{} {}", req.method(), req.uri().path())
}

/// A `tower::Layer` implementing the Go `withByIDTenantGuard` wrapper: before
/// the inner handler runs, the resource id (first path segment after
/// `prefix`) is verified against the caller's resolved tenant via
/// [`guard_resource_for_tenant`]. Route families attach it with
/// `route_layer` / `layer`.
#[derive(Clone)]
pub struct ByIDTenantGuardLayer {
    state: AppState,
    prefix: &'static str,
    table: &'static str,
    pk_column: &'static str,
    resource_kind: &'static str,
}

impl ByIDTenantGuardLayer {
    /// Creates the guard layer for a `/v1/<prefix>` route family.
    #[must_use]
    pub fn new(
        state: AppState,
        prefix: &'static str,
        table: &'static str,
        pk_column: &'static str,
        resource_kind: &'static str,
    ) -> Self {
        Self {
            state,
            prefix,
            table,
            pk_column,
            resource_kind,
        }
    }
}

impl<S> tower::Layer<S> for ByIDTenantGuardLayer {
    type Service = ByIDTenantGuard<S>;

    fn layer(&self, inner: S) -> Self::Service {
        ByIDTenantGuard {
            inner,
            state: self.state.clone(),
            prefix: self.prefix,
            table: self.table,
            pk_column: self.pk_column,
            resource_kind: self.resource_kind,
        }
    }
}

/// Service produced by [`ByIDTenantGuardLayer`].
#[derive(Clone)]
pub struct ByIDTenantGuard<S> {
    inner: S,
    state: AppState,
    prefix: &'static str,
    table: &'static str,
    pk_column: &'static str,
    resource_kind: &'static str,
}

impl<S> Service<Request> for ByIDTenantGuard<S>
where
    S: Service<Request, Response = Response, Error = Infallible> + Clone + Send + 'static,
    S::Future: Send + 'static,
{
    type Response = Response;
    type Error = Infallible;
    type Future = Pin<Box<dyn Future<Output = Result<Response, Infallible>> + Send>>;

    fn poll_ready(&mut self, cx: &mut Context<'_>) -> Poll<Result<(), Self::Error>> {
        self.inner.poll_ready(cx)
    }

    fn call(&mut self, req: Request) -> Self::Future {
        let mut inner = self.inner.clone();
        let state = self.state.clone();
        let prefix = self.prefix;
        let table = self.table;
        let pk_column = self.pk_column;
        let resource_kind = self.resource_kind;
        Box::pin(async move {
            let tenant = req.extensions().get::<TenantContext>().cloned();
            let id = id_from_path(req.uri().path(), prefix);
            let surface = surface_from_request(&req);
            if let Some(id) = id {
                if let Err(err) = guard_resource_for_tenant(
                    &state,
                    tenant.as_ref(),
                    &surface,
                    table,
                    pk_column,
                    id,
                    resource_kind,
                )
                .await
                {
                    return Ok(err.into_response());
                }
            }
            inner.call(req).await
        })
    }
}

/// Extracts the resource id from a request path relative to the route prefix:
/// the first segment after the prefix (Go `withByIDTenantGuard`). Returns
/// `None` when the path does not start with the prefix or has no id segment.
fn id_from_path<'a>(path: &'a str, prefix: &str) -> Option<&'a str> {
    let rest = path.strip_prefix(prefix)?;
    let id = rest.split('/').next().unwrap_or("");
    if id.is_empty() {
        None
    } else {
        Some(id)
    }
}
