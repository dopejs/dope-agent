//! Slack transport boundary (port of transport.go): the Transport trait, the
//! FakeTransport test double, and the mention-text helper.

use dope_imtypes::{OutboundReply, ReplyCapabilities, SentReply};
use parking_lot::Mutex;

use crate::error::SlackError;
use crate::route::InboundEvent;

/// The Slack channel transport boundary (Go `Transport`): starts the inbound
/// event pump and delivers outbound replies.
pub trait Transport: Send + Sync {
    /// Starts the transport, delivering each inbound event to handle. The
    /// handle is consumed synchronously during the call, matching Go's
    /// FakeTransport.Start which invokes the handler inline.
    fn start<'a>(&self, handle: Box<dyn Fn(InboundEvent) + 'a>) -> Result<(), SlackError>;
    fn send_reply(&self, reply: &OutboundReply) -> Result<SentReply, SlackError>;
    fn reply_capabilities(&self) -> ReplyCapabilities;
    fn close(&self) -> Result<(), SlackError>;
}

/// Test double transport (Go `FakeTransport`): records delivered replies and
/// can inject inbound events and reply failures.
#[derive(Default)]
pub struct FakeTransport {
    inner: Mutex<FakeTransportInner>,
}

#[derive(Default)]
struct FakeTransportInner {
    started: bool,
    inbound: Vec<InboundEvent>,
    sent: Vec<OutboundReply>,
    reply_err: Option<String>,
}

impl FakeTransport {
    /// Go `NewFakeTransport`: a fake seeded with inbound events.
    #[must_use]
    pub fn new(messages: Vec<InboundEvent>) -> Self {
        FakeTransport {
            inner: Mutex::new(FakeTransportInner {
                inbound: messages,
                ..FakeTransportInner::default()
            }),
        }
    }

    /// Go `SetReplyError`: makes send_reply fail with the given message.
    pub fn set_reply_error(&self, err: String) {
        self.inner.lock().reply_err = Some(err);
    }

    /// Go `SentReplies`: a copy of the delivered replies.
    #[must_use]
    pub fn sent_replies(&self) -> Vec<OutboundReply> {
        self.inner.lock().sent.clone()
    }
}

impl Transport for FakeTransport {
    fn start<'a>(&self, handle: Box<dyn Fn(InboundEvent) + 'a>) -> Result<(), SlackError> {
        let messages = {
            let mut inner = self.inner.lock();
            inner.started = true;
            inner.inbound.clone()
        };
        for message in messages {
            handle(message);
        }
        Ok(())
    }

    fn send_reply(&self, reply: &OutboundReply) -> Result<SentReply, SlackError> {
        let mut inner = self.inner.lock();
        if let Some(err) = &inner.reply_err {
            return Err(SlackError::Message(err.clone()));
        }
        inner.sent.push(reply.clone());
        Ok(SentReply {
            external_message_id: format!("slack_reply_{}", reply.channel_id.trim()),
        })
    }

    fn reply_capabilities(&self) -> ReplyCapabilities {
        ReplyCapabilities {
            max_message_length: 40000,
            ..ReplyCapabilities::default()
        }
    }

    fn close(&self) -> Result<(), SlackError> {
        Ok(())
    }
}

/// Go `NormalizeMentionText`: strips the bot mention from channel text.
#[must_use]
pub fn normalize_mention_text(text: &str, bot_user_id: &str) -> String {
    let mention = format!("<@{}>", bot_user_id.trim());
    text.replace(&mention, "").trim().to_string()
}

/// Go `UnsupportedSurfaceError`.
#[must_use]
pub fn unsupported_surface_error(surface: &str) -> SlackError {
    SlackError::UnsupportedSurface(surface.trim().to_string())
}
