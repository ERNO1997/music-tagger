# init-fingerprint-scan

Bootstrap the Go project per project.md's clean-architecture layout, and add a read-only local scan: recursively walk the mounted /music volume, compute an acoustic fingerprint for each .mp3/.flac file via fpcalc (no filename/tag-based matching), and expose the results via a GET endpoint and a minimal dark-mode web page listing them in a table. No AcoustID/MusicBrainz/Cover Art Archive/Genius calls, no tag writing, no file relocation yet.
