# add-file-tracking-store

Add a SQLite-backed store that persists per-file discovery/identification state (new/identified/not_found/missing), splitting scan into a fast DB-backed read and an explicit refresh action, and extending supported formats to .m4a.
