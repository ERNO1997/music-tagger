## 1. Baseline

- [ ] 1.1 Before making any change, manually exercise and note the current behavior of every UI flow in the real browser: table load, status/tagged/relocated/has-lyrics filters, search, column sort, pagination, select-page/select-all-matching, all four bulk actions, delete, details view (fields, lyrics, embedded tags, candidate picker, cover-browse picker, fingerprint), and all five polling loops — this is the baseline this change must reproduce exactly

## 2. Extract state and formatting

- [ ] 2.1 Create `ui/js/state.js`: move `filterState`, `sortState`, `pageState`, `selectedPaths`, `selectionMode`, `total`, `lastEntries`, and add `currentView` (default/only value: `'table'`)
- [ ] 2.2 Create `ui/js/format.js`: move `formatDuration`, `formatEta`, `escapeHtml`, `STATUS_LABELS`, `STATUS_CLASSES`, `DETAILS_FIELD_LABELS`, `EMBEDDED_TAG_FIELD_LABELS`, `IDENTIFY_ETA_THRESHOLD`

## 3. Extract API client

- [ ] 3.1 Create `ui/js/api.js`: one exported function per existing `fetch()` call site (list, scan trigger/status, identify trigger/status/resolve, enrich trigger/status, tag trigger/status, relocate trigger/status, cover/candidates/choose, lyrics, embedded tags, fingerprint, candidates, delete)

## 4. Extract table rendering

- [ ] 4.1 Create `ui/js/table.js`: move `renderTable`, `renderRow`, `renderMetadataCell`, `renderCoverCell`, `renderTaggedCell`, `renderRelocatedCell`, `renderActionsCell`, `updateSelectionBanner`, `updatePaginationControls`, `updateSortIndicators`

## 5. Extract details view

- [ ] 5.1 Create `ui/js/details.js`: move `openDetails`, `closeDetails`, `loadFingerprint`, `loadEmbeddedTags`, `loadLyrics`, `loadCandidates`, `resolveCandidate`, `loadCoverCandidates`, `chooseCover`, and the details-related DOM element references

## 6. Extract polling into one parameterized helper

- [ ] 6.1 Create `ui/js/polling.js`: implement `pollJob({ statusUrl, onUpdate, intervalMs })` and replace the five `start*Polling`/`set*UI` function pairs with five calls to it from `main.js`, preserving each job's exact current update/UI behavior

## 7. Extract bulk actions

- [ ] 7.1 Create `ui/js/actions.js`: move the identify/enrich/tag/relocate/delete trigger functions and their button-state update functions (`updateAllActionButtons`, `updateIdentifyButton`, etc.)

## 8. Wire it together

- [ ] 8.1 Create `ui/js/main.js`: DOM element lookups not already covered by `table.js`/`details.js`, all event-listener wiring, `renderCurrentView` (switching on `state.currentView`, one case: `'table'`), initial `loadLibrary()` call
- [ ] 8.2 Delete `ui/js/app.js`
- [ ] 8.3 Update `ui/index.html`'s script tag to `<script type="module" src="/js/main.js"></script>`

## 9. Verification

- [ ] 9.1 Run `go build ./...` and `go vet ./...` inside Docker (confirms `ui/embed.go`'s existing `//go:embed js` still embeds the new module files correctly)
- [ ] 9.2 Re-run every flow noted in the 1.1 baseline against the refactored code and confirm identical behavior — table load, all filters, search, sort, pagination, selection modes, all four bulk actions, delete, every details-view section, all five polling loops
- [ ] 9.3 Open the browser devtools console during the full pass in 9.2 and confirm no module-loading errors, import errors, or JS exceptions
- [ ] 9.4 Confirm no regressions specifically in the candidate picker and cover-browse picker (built most recently, most likely to have subtle state dependencies on the surrounding monolith)
