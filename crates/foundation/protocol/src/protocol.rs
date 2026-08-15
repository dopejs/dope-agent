use serde::Deserialize;
use serde::Serialize;

/// Client → core submission. Mirrors codex's `Op`: every way a caller can
/// drive or interrupt the agent goes through this enum.
#[derive(Debug, Clone, PartialEq, Serialize, Deserialize)]
#[serde(tag = "op", rename_all = "snake_case")]
pub enum Op {
    UserInput { text: String },
    Interrupt,
    Shutdown,
}

/// One item of conversation history fed back to the model.
#[derive(Debug, Clone, PartialEq, Serialize, Deserialize)]
#[serde(tag = "type", rename_all = "snake_case")]
pub enum ResponseItem {
    Message {
        role: Role,
        content: String,
    },
    FunctionCall {
        call_id: String,
        name: String,
        arguments: String,
    },
    FunctionCallOutput {
        call_id: String,
        output: String,
    },
}

#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize)]
#[serde(rename_all = "lowercase")]
pub enum Role {
    System,
    User,
    Assistant,
    Tool,
}

/// core → client event. `id` is a per-turn sequence anchor so clients can
/// order and deduplicate deltas.
#[derive(Debug, Clone, PartialEq, Serialize, Deserialize)]
pub struct Event {
    pub id: String,
    pub msg: EventMsg,
}

#[derive(Debug, Clone, PartialEq, Serialize, Deserialize)]
#[serde(tag = "type", rename_all = "snake_case")]
pub enum EventMsg {
    TurnStarted,
    AgentMessageDelta {
        delta: String,
    },
    ToolCallBegin {
        call_id: String,
        name: String,
        arguments: String,
    },
    ToolCallEnd {
        call_id: String,
        output: String,
        success: bool,
    },
    AgentMessage {
        message: String,
    },
    TurnComplete,
    Error {
        message: String,
    },
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn op_serializes_with_snake_case_tag() {
        let op = Op::UserInput { text: "hi".into() };
        let json = serde_json::to_value(&op).unwrap();
        assert_eq!(json, serde_json::json!({"op": "user_input", "text": "hi"}));
    }

    #[test]
    fn event_msg_round_trips() {
        let event = Event {
            id: "t-1".into(),
            msg: EventMsg::ToolCallEnd {
                call_id: "call_1".into(),
                output: "ok".into(),
                success: true,
            },
        };
        let json = serde_json::to_string(&event).unwrap();
        let back: Event = serde_json::from_str(&json).unwrap();
        assert_eq!(event, back);
    }

    #[test]
    fn response_item_tool_cycle_shape() {
        let item = ResponseItem::FunctionCall {
            call_id: "call_1".into(),
            name: "shell".into(),
            arguments: "{}".into(),
        };
        let json = serde_json::to_value(&item).unwrap();
        assert_eq!(json["type"], "function_call");
        assert_eq!(json["name"], "shell");
    }
}
