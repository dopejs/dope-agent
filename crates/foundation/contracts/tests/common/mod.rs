//! Shared helpers for the ported daemon contract-fixture tests (wave 8).
//!
//! Mirrors the Go helpers in daemon/internal/contracts: schemaRootDir
//! and mustValidateFixtures, plus the shared data module holding the
//! fixture maps that several Go test files reference.

use std::path::PathBuf;

pub mod data;

pub type Fixture = (&'static str, &'static str);

/// Mirrors Go's schemaRootDir: the repository root containing the
/// schemas/ tree.
pub fn schema_root_dir() -> PathBuf {
    let root = PathBuf::from(env!("CARGO_MANIFEST_DIR"))
        .join("..")
        .join("..")
        .join("..");
    assert!(
        root.join("schemas").is_dir(),
        "schemas/ directory not found under {}",
        root.display()
    );
    root
}

/// Mirrors Go's mustValidateFixtures: every schemaPath -> fixture pair must
/// validate cleanly.
pub fn validate_fixtures(validator: &dope_contracts::Validator, fixtures: &[Fixture]) {
    for (schema_path, fixture) in fixtures {
        validator
            .validate_relative(schema_path, fixture.as_bytes())
            .unwrap_or_else(|err| panic!("ValidateRelative({schema_path}): {err}"));
    }
}
