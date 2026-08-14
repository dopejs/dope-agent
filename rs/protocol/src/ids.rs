use std::fmt;
use std::str::FromStr;

use serde::Deserialize;
use serde::Serialize;
use uuid::Uuid;

macro_rules! define_id {
    ($($(#[$meta:meta])* $name:ident),+ $(,)?) => {
        $(
            $(#[$meta])*
            ///
            /// Generated IDs are UUIDv7; some use cases rely on time ordering.
            #[derive(Debug, Clone, Copy, PartialEq, Eq, Hash)]
            pub struct $name(Uuid);

            impl $name {
                pub fn new() -> Self {
                    Self(Uuid::now_v7())
                }

                pub fn from_uuid(uuid: Uuid) -> Self {
                    Self(uuid)
                }

                pub fn as_uuid(&self) -> &Uuid {
                    &self.0
                }
            }

            impl Default for $name {
                fn default() -> Self {
                    Self::new()
                }
            }

            impl fmt::Display for $name {
                fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
                    self.0.fmt(f)
                }
            }

            impl FromStr for $name {
                type Err = uuid::Error;

                fn from_str(s: &str) -> Result<Self, Self::Err> {
                    Ok(Self(Uuid::parse_str(s)?))
                }
            }

            impl TryFrom<&str> for $name {
                type Error = uuid::Error;

                fn try_from(value: &str) -> Result<Self, Self::Error> {
                    value.parse()
                }
            }

            impl Serialize for $name {
                fn serialize<S: serde::Serializer>(&self, serializer: S) -> Result<S::Ok, S::Error> {
                    serializer.collect_str(self)
                }
            }

            impl<'de> Deserialize<'de> for $name {
                fn deserialize<D: serde::Deserializer<'de>>(deserializer: D) -> Result<Self, D::Error> {
                    let s = String::deserialize(deserializer)?;
                    s.parse().map_err(serde::de::Error::custom)
                }
            }
        )+
    };
}

define_id!(
    /// Tenant scope for every persisted resource and permission check.
    TenantId,
    /// Conversation thread; the primary runtime-evidence anchor.
    ThreadId,
    /// A single agent run within a thread.
    RunId,
    /// Agent profile/persona configuration record.
    ProfileId,
);

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn ids_round_trip_through_strings() {
        let id = ThreadId::new();
        let s = id.to_string();
        let parsed: ThreadId = s.parse().unwrap();
        assert_eq!(id, parsed);
    }

    #[test]
    fn ids_round_trip_through_json() {
        let id = RunId::new();
        let json = serde_json::to_string(&id).unwrap();
        let back: RunId = serde_json::from_str(&json).unwrap();
        assert_eq!(id, back);
        assert_eq!(json, format!("\"{id}\""));
    }

    #[test]
    fn ids_reject_garbage() {
        assert!("not-a-uuid".parse::<TenantId>().is_err());
    }

    #[test]
    fn v7_ids_are_time_ordered() {
        let a = ThreadId::new();
        let b = ThreadId::new();
        let ts = |id: ThreadId| id.as_uuid().get_timestamp().unwrap().to_unix();
        assert!(ts(a) <= ts(b));
    }
}
