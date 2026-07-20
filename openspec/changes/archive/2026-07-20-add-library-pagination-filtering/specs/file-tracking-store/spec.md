## ADDED Requirements

### Requirement: Filtered, sorted, and paginated reads
The system SHALL support reading tracked records filtered by effective status, tagged outcome, and/or relocated outcome; searched by a case-insensitive substring match against path, artist, album, and title; sorted by an allow-listed set of columns (path, status, artist, album, duration, year) in ascending or descending order with a deterministic tie-break so repeated reads against unchanged data return the same order; and paginated by a result limit and offset — reporting the total number of matching records independent of the page size. This is distinct from the full, unfiltered table load used internally for scan change-detection, which is unaffected by this requirement.

#### Scenario: Filtering narrows the result set
- **WHEN** a read is requested with a status, tagged, or relocated filter
- **THEN** only records matching that filter SHALL be included, and the reported total SHALL reflect only the matching count

#### Scenario: Search matches across multiple fields
- **WHEN** a read is requested with a search term
- **THEN** records whose path, artist, album, or title contains that term, case-insensitively, SHALL be included, and records matching none of those fields SHALL be excluded

#### Scenario: Sorting is stable under concurrent writes
- **WHEN** a read is requested with a sort column and a background job is concurrently modifying tracked records
- **THEN** the returned order SHALL be deterministic for any given snapshot of the data, using a stable tie-break so records are not silently duplicated or skipped across repeated reads purely due to sort-key ties

#### Scenario: Pagination reports the total independent of page size
- **WHEN** a read is requested with a limit and offset
- **THEN** the number of records returned SHALL be at most the limit, and the reported total SHALL reflect the full count of matching records, not the count on the current page

#### Scenario: Resolving a filter to a bare path list
- **WHEN** the full set of paths matching a filter is requested, without pagination
- **THEN** the system SHALL return every currently-matching path, ignoring any limit or offset, for use in resolving a bulk action's filter-based selection at the moment it executes
