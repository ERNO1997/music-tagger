async function postTrigger(url, body, label) {
  const res = await fetch(url, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(body),
  });
  if (res.status !== 202 && res.status !== 409) {
    const errBody = await res.json().catch(() => ({}));
    throw new Error(errBody.error || `${label} request failed: ${res.status}`);
  }
}

export async function fetchLibrary(params) {
  const res = await fetch(`/api/v1/library?${params.toString()}`);
  if (!res.ok) {
    throw new Error(`request failed: ${res.status}`);
  }
  return res.json();
}

export async function fetchSelection(body, params) {
  const res = await fetch(`/api/v1/library/selection?${params.toString()}`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(body),
  });
  if (!res.ok) {
    const errBody = await res.json().catch(() => ({}));
    throw new Error(errBody.error || `request failed: ${res.status}`);
  }
  return res.json();
}

export async function fetchTree(params) {
  const res = await fetch(`/api/v1/library/tree?${params.toString()}`);
  if (!res.ok) {
    throw new Error(`request failed: ${res.status}`);
  }
  return res.json();
}

export async function fetchArtists(params) {
  const res = await fetch(`/api/v1/library/artists?${params.toString()}`);
  if (!res.ok) {
    throw new Error(`request failed: ${res.status}`);
  }
  return res.json();
}

export async function fetchAlbums(params) {
  const res = await fetch(`/api/v1/library/albums?${params.toString()}`);
  if (!res.ok) {
    throw new Error(`request failed: ${res.status}`);
  }
  return res.json();
}

export async function fetchTracks(params) {
  const res = await fetch(`/api/v1/library/tracks?${params.toString()}`);
  if (!res.ok) {
    throw new Error(`request failed: ${res.status}`);
  }
  return res.json();
}

export async function deleteLibraryEntry(path) {
  const res = await fetch(`/api/v1/library/entry?path=${encodeURIComponent(path)}`, { method: 'DELETE' });
  if (res.status !== 204 && res.status !== 200) {
    const body = await res.json().catch(() => ({}));
    throw new Error(body.error || `delete request failed: ${res.status}`);
  }
}

export async function fetchCandidates(path) {
  const res = await fetch(`/api/v1/library/candidates?path=${encodeURIComponent(path)}`);
  if (!res.ok) {
    throw new Error(`request failed: ${res.status}`);
  }
  const data = await res.json();
  return data.candidates || [];
}

export async function searchIdentify(path, query) {
  const res = await fetch('/api/v1/library/identify/search', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ path, query }),
  });
  if (!res.ok) {
    const body = await res.json().catch(() => ({}));
    throw new Error(body.error || `search request failed: ${res.status}`);
  }
  const data = await res.json();
  return data.candidates || [];
}

export async function postIdentifyResolve(path, recordingMbid) {
  const res = await fetch('/api/v1/library/identify/resolve', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ path, recording_mbid: recordingMbid }),
  });
  if (!res.ok) {
    const body = await res.json().catch(() => ({}));
    throw new Error(body.error || `resolve request failed: ${res.status}`);
  }
}

export async function fetchCoverCandidates(path) {
  const res = await fetch(`/api/v1/library/cover/candidates?path=${encodeURIComponent(path)}`);
  if (!res.ok) {
    throw new Error(`request failed: ${res.status}`);
  }
  const data = await res.json();
  return data.candidates || [];
}

export async function postCoverChoose(path, releaseMbid, imageUrl) {
  const res = await fetch('/api/v1/library/cover/choose', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ path, release_mbid: releaseMbid, image_url: imageUrl }),
  });
  if (!res.ok) {
    const body = await res.json().catch(() => ({}));
    throw new Error(body.error || `choose request failed: ${res.status}`);
  }
}

export async function fetchFingerprint(path) {
  const res = await fetch(`/api/v1/library/fingerprint?path=${encodeURIComponent(path)}`);
  if (!res.ok) {
    throw new Error(`request failed: ${res.status}`);
  }
  return res.json();
}

export async function fetchEmbeddedTags(path) {
  const res = await fetch(`/api/v1/library/tags?path=${encodeURIComponent(path)}`);
  if (!res.ok) {
    throw new Error(`request failed: ${res.status}`);
  }
  return res.json();
}

export async function fetchLyrics(path) {
  const res = await fetch(`/api/v1/library/lyrics?path=${encodeURIComponent(path)}`);
  if (!res.ok) {
    throw new Error(`request failed: ${res.status}`);
  }
  return res.json();
}

export async function fetchScanStatus() {
  const res = await fetch('/api/v1/library/scan/status');
  if (!res.ok) {
    throw new Error(`status request failed: ${res.status}`);
  }
  return res.json();
}

export async function fetchIdentifyStatus() {
  const res = await fetch('/api/v1/library/identify/status');
  if (!res.ok) {
    throw new Error(`status request failed: ${res.status}`);
  }
  return res.json();
}

export async function fetchEnrichStatus() {
  const res = await fetch('/api/v1/library/enrich/status');
  if (!res.ok) {
    throw new Error(`status request failed: ${res.status}`);
  }
  return res.json();
}

export async function fetchTagStatus() {
  const res = await fetch('/api/v1/library/tag/status');
  if (!res.ok) {
    throw new Error(`status request failed: ${res.status}`);
  }
  return res.json();
}

export async function fetchRelocateStatus() {
  const res = await fetch('/api/v1/library/relocate/status');
  if (!res.ok) {
    throw new Error(`status request failed: ${res.status}`);
  }
  return res.json();
}

export async function postScanTrigger() {
  const res = await fetch('/api/v1/library/scan', { method: 'POST' });
  if (res.status !== 202 && res.status !== 409) {
    throw new Error(`refresh request failed: ${res.status}`);
  }
}

export async function postIdentifyTrigger(body) {
  await postTrigger('/api/v1/library/identify', body, 'identify');
}

export async function postEnrichTrigger(body) {
  await postTrigger('/api/v1/library/enrich', body, 'enrich');
}

export async function postTagTrigger(body) {
  await postTrigger('/api/v1/library/tag', body, 'tag');
}

export async function postRelocateTrigger(body) {
  await postTrigger('/api/v1/library/relocate', body, 'relocate');
}
