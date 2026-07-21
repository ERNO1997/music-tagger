## ADDED Requirements

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
