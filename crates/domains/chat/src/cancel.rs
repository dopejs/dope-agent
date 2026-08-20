//! Cancellation token standing in for Go's `context.Context` cancel funcs.
//!
//! The Go chat service receives a `context.Context` whose cancellation aborts
//! the in-flight LLM dispatch. Go's `context.WithCancel` semantics are parent
//! -> child: cancelling a parent cancels every descendant, while cancelling a
//! child only affects that subtree. This token mirrors that shape
//! (`AtomicBool` + `child()`/`kill()`), and `link_to` bridges a token into the
//! `kura_llm::CancelToken` the async dispatcher polls, so killing the chat
//! token aborts the blocking dispatch.

use std::sync::atomic::{AtomicBool, Ordering};
use std::sync::{Arc, Weak};

use parking_lot::Mutex;

/// One kill watcher registered via [`CancellationToken::link_to`].
type Watcher = Box<dyn FnOnce() + Send + 'static>;

#[derive(Default)]
struct TokenState {
    cancelled: AtomicBool,
    watchers: Mutex<Vec<(usize, Watcher)>>,
    next_watcher_id: std::sync::atomic::AtomicUsize,
    children: Mutex<Vec<Weak<TokenState>>>,
}

/// A cancellable signal shared with dispatched work.
///
/// Cheap to clone: every clone observes the same state. Killing the parent
/// cancels all descendants; killing a child cancels only its subtree.
#[derive(Clone, Default)]
pub struct CancellationToken {
    state: Arc<TokenState>,
}

impl CancellationToken {
    /// A fresh, not-yet-cancelled token.
    #[must_use]
    pub fn new() -> Self {
        Self::default()
    }

    /// Go `context.WithCancel`: returns a child token linked to this parent.
    /// Killing the parent kills the child (and its descendants); killing the
    /// child alone leaves the parent untouched.
    #[must_use]
    pub fn child(&self) -> CancellationToken {
        let state = Arc::new(TokenState::default());
        self.state.children.lock().push(Arc::downgrade(&state));
        CancellationToken { state }
    }

    /// Go `cancel()`: marks the token cancelled, runs its kill watchers once,
    /// and recursively kills every child.
    pub fn kill(&self) {
        if self.state.cancelled.swap(true, Ordering::SeqCst) {
            return;
        }
        let watchers = std::mem::take(&mut *self.state.watchers.lock());
        for (_, watcher) in watchers {
            watcher();
        }
        let children: Vec<Arc<TokenState>> = {
            self.state
                .children
                .lock()
                .iter()
                .filter_map(|weak| weak.upgrade())
                .collect()
        };
        for child in children {
            CancellationToken { state: child }.kill();
        }
    }

    /// Whether [`CancellationToken::kill`] has run.
    #[must_use]
    pub fn is_cancelled(&self) -> bool {
        self.state.cancelled.load(Ordering::SeqCst)
    }

    /// Registers a one-shot watcher invoked by [`CancellationToken::kill`]. The
    /// returned [`KillLink`] unregisters the watcher when dropped, so a token
    /// cancelled after the linked work finished cannot touch it.
    pub fn register_watcher(&self, watcher: impl FnOnce() + Send + 'static) -> KillLink<'_> {
        let id = self.state.next_watcher_id.fetch_add(1, Ordering::Relaxed);
        self.state.watchers.lock().push((id, Box::new(watcher)));
        KillLink {
            state: Arc::clone(&self.state),
            id,
            _lifetime: std::marker::PhantomData,
        }
    }

    /// Bridges this token into a [`kura_llm::CancelToken`]: killing this token
    /// cancels the LLM token, which aborts the async dispatcher polled inside
    /// the service's sync bridge.
    pub fn link_to(&self, token: &kura_llm::CancelToken) -> KillLink<'_> {
        let target = token.clone();
        self.register_watcher(move || {
            target.cancel();
        })
    }
}

impl std::fmt::Debug for CancellationToken {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        f.debug_struct("CancellationToken")
            .field("cancelled", &self.is_cancelled())
            .finish()
    }
}

/// Handle returned by [`CancellationToken::register_watcher`] and
/// [`CancellationToken::link_to`]; dropping it unregisters the watcher.
pub struct KillLink<'a> {
    state: Arc<TokenState>,
    id: usize,
    _lifetime: std::marker::PhantomData<&'a ()>,
}

impl Drop for KillLink<'_> {
    fn drop(&mut self) {
        self.state.watchers.lock().retain(|(id, _)| *id != self.id);
    }
}
