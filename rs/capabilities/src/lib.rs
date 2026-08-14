//! Port of `daemon/internal/capabilities`: supervised child-process capability
//! registration, health/failure/restart supervision, and the integration-adapter runtime
//! bridge (Roadmap 59).

mod adapter_runtime;
mod supervisor;

pub use adapter_runtime::{
    start_adapter_runtime, AdapterHealthEvent, AdapterRuntime, Readiness, RuntimeError,
    KIND_INTEGRATION_ADAPTER,
};
pub use supervisor::{
    Capability, RegisterInput, ReportFailureInput, ReportHealthInput, Status, Supervisor,
    SupervisorError,
};
