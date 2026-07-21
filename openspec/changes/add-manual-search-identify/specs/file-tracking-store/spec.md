## MODIFIED Requirements

### Requirement: Ambiguous identification is recorded with candidate metadata
The system SHALL treat an AcoustID lookup whose accepted (at-or-above-confidence-threshold) top result ties two or more recordings that resolve to distinct artist/title identities as needing human disambiguation rather than picking one automatically: it SHALL resolve each tied recording's canonical metadata, store the full set as that file's candidates, and set the file's status to `ambiguous` without writing any single resolved-metadata field to the file's own record. A file's stored candidates MAY also originate from a manual search rather than AcoustID tied-recordings — both are stored and resolved through the same mechanism, since the resulting "several candidates, pick one" state is identical regardless of source.

#### Scenario: Tied recordings resolving to distinct identities are recorded as ambiguous
- **WHEN** identification's AcoustID lookup returns an accepted top result tied to recordings that resolve to two or more distinct (artist, title) identities
- **THEN** the system SHALL set that file's status to `ambiguous`, store every distinct resolved candidate, and SHALL NOT write resolved metadata to the file's own record

#### Scenario: Tied recordings resolving to the same identity are recorded as a normal success
- **WHEN** identification's AcoustID lookup returns an accepted top result tied to recordings that all resolve to the same (artist, title) identity
- **THEN** the system SHALL set that file's status to `identified` and record that shared identity's resolved metadata, exactly as if AcoustID had returned only one recording

#### Scenario: An ambiguous file's candidates are retrievable
- **WHEN** a file has been recorded `ambiguous`
- **THEN** the system SHALL make its full stored candidate list (each candidate's resolved artist, album, title, track number, and other metadata) available for retrieval

#### Scenario: A manual search's results are recorded the same way, for a file in any prior status
- **WHEN** a manual search for a tracked file returns one or more candidates, regardless of whether that file's prior status was `new`, `not_found`, `identified`, or `ambiguous`
- **THEN** the system SHALL discard the file's prior resolved metadata and any previously stored candidates, store the search's results as its new candidates, and set its status to `ambiguous`

#### Scenario: A manual search with no results does not alter the file's prior state
- **WHEN** a manual search for a tracked file returns zero candidates
- **THEN** the system SHALL leave that file's status, resolved metadata, and any previously stored candidates unchanged

### Requirement: A stored candidate can be chosen to resolve an ambiguous file
The system SHALL allow a stored candidate to be selected for a tracked file whose status is `ambiguous`, recording that choice exactly as a normal successful identification and discarding the file's other stored candidates. This applies uniformly regardless of whether the file's candidates originated from AcoustID tied-recordings or a manual search.

#### Scenario: Choosing a valid candidate resolves the file
- **WHEN** a candidate matching one of an `ambiguous` file's stored recording IDs is chosen
- **THEN** the system SHALL set that file's status to `identified`, store the chosen candidate's resolved metadata exactly as a normal successful identification would, and discard its other stored candidates

#### Scenario: Choosing an unrecognized candidate is rejected
- **WHEN** a candidate recording ID is submitted for a file that does not have a stored candidate with that ID
- **THEN** the system SHALL leave that file's status and stored candidates unchanged and SHALL report that the requested candidate was not found

#### Scenario: Choosing a candidate that originated from a manual search
- **WHEN** a candidate that was stored via a manual search (rather than AcoustID tied-recordings) is chosen
- **THEN** the system SHALL resolve it identically to choosing an AcoustID-sourced candidate — same recorded fields, same downstream tagging/relocation eligibility
