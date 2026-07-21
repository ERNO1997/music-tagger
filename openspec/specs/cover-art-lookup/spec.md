## Purpose

Resolving a MusicBrainz Release ID (found via the `musicbrainz-metadata` capability) to a front-cover image via Cover Art Archive — a fully public, unauthenticated API with no documented rate limit, falling back to the release-group when the specific release has no art uploaded.

## Requirements

### Requirement: Front cover resolution via Cover Art Archive
Given a MusicBrainz Release ID and Release-Group ID, the system SHALL query Cover Art Archive for the release's images and resolve the front cover, preferring an image explicitly marked as front and falling back to the first image if none is marked. If the specific release has no cover art, the system SHALL fall back to querying Cover Art Archive's release-group endpoint before treating the lookup as "no cover art available".

#### Scenario: Front cover found on the specific release
- **WHEN** Cover Art Archive returns one or more images for a Release ID, at least one marked `front`
- **THEN** the system SHALL download that image's bytes at the "large" (~500px) size

#### Scenario: No image explicitly marked front
- **WHEN** Cover Art Archive returns images for a release but none is marked `front`
- **THEN** the system SHALL download the first image returned

#### Scenario: Specific release has no art, but a sibling release does
- **WHEN** Cover Art Archive returns a 404 for the specific Release ID, but its release-group endpoint resolves to at least one image
- **THEN** the system SHALL download the resolved image rather than treating the release-level 404 as "no cover art available"

#### Scenario: No cover art available anywhere in the release-group
- **WHEN** Cover Art Archive returns a 404 for both the specific Release ID and its release-group
- **THEN** the system SHALL treat this as "no cover art available", not an error, and SHALL NOT alter the file's identification status

#### Scenario: Cover Art Archive request failure
- **WHEN** a Cover Art Archive request fails for a reason other than a 404 (network error, non-404 non-2xx response, or malformed response)
- **THEN** the system SHALL return an error distinguishable from "no cover art available"

### Requirement: Requests always use HTTPS
The system SHALL request cover art over HTTPS regardless of the scheme present in Cover Art Archive's response data.

#### Scenario: Response contains an HTTP URL
- **WHEN** Cover Art Archive's image metadata contains an `http://` URL
- **THEN** the system SHALL upgrade the request to `https://` before downloading

### Requirement: Browsable front-cover listing across a release-group's sibling editions
The system SHALL support listing a single release's front-cover thumbnail and image URLs without downloading its bytes, and downloading arbitrary previously-listed image bytes on request — a separate capability from the existing single automatic cover-art resolution, used to let a user browse and choose among a release-group's sibling editions' covers rather than always accepting the one release a recording resolved to.

#### Scenario: A release's front image is listed
- **WHEN** a release's front-cover listing is requested and Cover Art Archive has an image marked `front` for that release
- **THEN** the system SHALL return that image's thumbnail and full-size URLs without downloading the image bytes

#### Scenario: A release has no front image
- **WHEN** a release's front-cover listing is requested and Cover Art Archive has no image marked `front` for that release (including a 404)
- **THEN** the system SHALL report that no image was found, distinct from a returned error

#### Scenario: Front-cover listing request failure
- **WHEN** a front-cover listing request fails for a reason other than "not found" (network error, non-404 non-2xx response, or malformed response)
- **THEN** the system SHALL return an error distinguishable from "no image found"

#### Scenario: Downloading a previously-listed image
- **WHEN** an image download is requested using a URL previously returned by a front-cover listing
- **THEN** the system SHALL fetch and return that image's bytes

#### Scenario: Downloading a URL outside Cover Art Archive's own host is refused
- **WHEN** an image download is requested using a URL whose host is not Cover Art Archive's own domain
- **THEN** the system SHALL refuse the request with an error and SHALL NOT attempt to fetch it
