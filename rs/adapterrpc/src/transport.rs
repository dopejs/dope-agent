//! Serialized request/response round trip over a newline-delimited stdio stream.
//! Mirrors `transport.go`.

use std::io::{BufReader, Read, Write};
use std::sync::atomic::{AtomicBool, Ordering};
use std::sync::mpsc;
use std::thread;
use std::time::Duration;

use parking_lot::Mutex;

use crate::codec::{read_response, write_message};
use crate::error::{CodecError, Error};
use crate::types::{Request, Response};

/// Carries one request/response round trip. Calls are serialized: a single stdio stream
/// cannot interleave frames, and serialization also guarantees per-call isolation across
/// concurrent callers (FR-015) at the wire.
///
/// Once a round trip times out or hits a stream error, the transport is "broken": a leaked
/// reader may still be consuming the stream, and the frame stream may be desynchronized, so
/// every subsequent call fails fast with [`Error::Unreachable`] instead of spawning a
/// competing reader on the same buffered reader. Recovery requires a fresh client
/// (supervisor restart / respawn).
pub(crate) struct Transport {
    mu: Mutex<()>,
    w: Mutex<Box<dyn Write + Send>>,
    r: std::sync::Arc<Mutex<BufReader<Box<dyn Read + Send>>>>,
    broken: AtomicBool,
}

impl Transport {
    pub(crate) fn new(w: Box<dyn Write + Send>, r: Box<dyn Read + Send>) -> Self {
        Transport {
            mu: Mutex::new(()),
            w: Mutex::new(w),
            r: std::sync::Arc::new(Mutex::new(BufReader::new(r))),
            broken: AtomicBool::new(false),
        }
    }

    /// Write the request and wait for the correlated response, honoring `timeout`. On
    /// deadline expiry it returns [`Error::Timeout`] (hang/timeout classification,
    /// FR-007b) and poisons the transport; the caller treats that as an unconfirmed
    /// outcome.
    pub(crate) fn round_trip(&self, req: &Request, timeout: Duration) -> Result<Response, Error> {
        let _guard = self.mu.lock();

        if self.broken.load(Ordering::SeqCst) {
            return Err(Error::Unreachable);
        }

        if write_message(&mut *self.w.lock(), req).is_err() {
            self.broken.store(true, Ordering::SeqCst);
            return Err(Error::Unreachable);
        }

        let reader = std::sync::Arc::clone(&self.r);
        let (tx, rx) = mpsc::channel();
        thread::spawn(move || {
            let res = read_response(&mut *reader.lock());
            // If the round trip timed out, the receiver is gone; this thread simply
            // drains when the adapter eventually responds or the stream closes.
            let _ = tx.send(res);
        });

        match rx.recv_timeout(timeout) {
            Ok(Ok(resp)) => Ok(resp),
            Ok(Err(err)) => {
                self.broken.store(true, Ordering::SeqCst);
                match err {
                    // Stream died (process exit, closed pipe): unreachable.
                    CodecError::Io(e)
                        if e.kind() == std::io::ErrorKind::UnexpectedEof
                            || e.kind() == std::io::ErrorKind::BrokenPipe =>
                    {
                        Err(Error::Unreachable)
                    }
                    other => Err(Error::MalformedResponse(other.to_string())),
                }
            }
            Err(mpsc::RecvTimeoutError::Timeout) => {
                // Deadline expiry: the outcome is unconfirmed. Poison the transport so the
                // leaked reader is never raced by a subsequent call.
                self.broken.store(true, Ordering::SeqCst);
                Err(Error::Timeout)
            }
            Err(mpsc::RecvTimeoutError::Disconnected) => {
                // Reader thread panicked before sending.
                self.broken.store(true, Ordering::SeqCst);
                Err(Error::Unreachable)
            }
        }
    }
}
