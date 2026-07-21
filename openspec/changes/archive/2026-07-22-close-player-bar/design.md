## Context

`usePlayer.js` holds one reactive `playerState` object (`visible`, `src`, `title`, `artist`) shared by every view's play button and `PlayerBar.vue`. `playTrack(entry)` is the only exported mutator today; nothing ever sets `visible` back to `false`. `PlayerBar.vue` renders with `v-show="playerState.visible"`, so the `<audio>` element itself stays mounted (and keeps its loaded track) even while hidden — that's already the right foundation for a close button; it just needs a way to actually flip `visible` back off and stop the audio.

## Goals / Non-Goals

**Goals:**
- A close control on the player bar that stops playback and hides the bar.
- Playing a new track after closing brings the bar back exactly as it does today (no special-casing needed elsewhere, since every view's play button already just calls `playTrack()`).

**Non-Goals:**
- A "recently played" or history list — out of scope, not asked for.
- Persisting closed/open state across a page reload — the player already doesn't survive a reload today (no track resumes on load), so closing doesn't need to either.

## Decisions

### `closePlayer()` pauses the audio element and resets `visible`
Add `closePlayer()` to `usePlayer.js`, mirroring `playTrack()`'s existing pattern of mutating the shared `playerState`. Since `PlayerBar.vue` already holds the `<audio>` ref (`audioRef`) for its `nextTick()`-then-`play()` logic, the close button's click handler lives in `PlayerBar.vue` itself: it calls `audioRef.value.pause()` directly (stopping playback immediately, not waiting on reactivity), then `closePlayer()` (setting `playerState.visible = false`). Alternative considered: pausing via a watcher on `playerState.visible` in `usePlayer.js` instead of a direct DOM call in the component — rejected as an unnecessary indirection when the component already owns the audio element reference for exactly this kind of imperative call.

### `src` is left as-is on close, not cleared
Closing sets `visible = false` and pauses, but does not reset `playerState.src`/`title`/`artist` to empty. This keeps the change minimal (one new boolean flip, no extra "did we clear everything" bookkeeping) and has no observable difference from clearing, since the bar is hidden either way and `playTrack()` always overwrites `src`/`title`/`artist` on the next play regardless of their prior value.

### `playTrack()` replaying the same track is watched via a counter, not `src`
Discovered during verification, not anticipated up front: `PlayerBar.vue`'s existing play-on-load logic was a `watch(() => playerState.src, ...)`. Vue's `watch()` doesn't invoke its callback when the watched value is set to what it already was — so closing a track and then replaying that *same* track (an entirely expected flow: close, then hit play on the same row again) silently did nothing, since `src` was unchanged. Fixed by adding `playerState.playToken` (an integer, incremented on every `playTrack()` call) and watching that instead — it always changes, even for a repeat play of the same path. The watcher also now resets `audioRef.value.currentTime = 0` before calling `.play()`, matching the pre-Vue-port behavior (assigning `.src` on a real `<audio>` element always reloads it from 0, even when reassigned to an identical value — a side effect Vue's attribute-diffing `:src` binding doesn't reproduce on its own).

## Risks / Trade-offs

- **[Trade-off] `playToken` is a small piece of state whose only job is "not being `src`"** → Accepted: it's the simplest fix that doesn't change `playTrack()`'s external behavior or `PlayerBar.vue`'s ownership of the audio element; an alternative (e.g., a callback/event instead of a watched reactive value) would be more machinery for the same result.
