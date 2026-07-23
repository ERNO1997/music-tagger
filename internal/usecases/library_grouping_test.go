package usecases

import "testing"

func findArtist(t *testing.T, summaries []ArtistSummary, key string) ArtistSummary {
	t.Helper()
	for _, s := range summaries {
		if s.Key == key {
			return s
		}
	}
	t.Fatalf("no artist summary with key %q in %+v", key, summaries)
	return ArtistSummary{}
}

func TestGroupArtists_SameNameDifferentMBIDsStaySeparate(t *testing.T) {
	rows := []ArtistRow{
		{Artist: "Overlap", ArtistMBID: "mbid-a"},
		{Artist: "Overlap", ArtistMBID: "mbid-b"},
	}
	got := GroupArtists(rows)
	if len(got) != 2 {
		t.Fatalf("len(got) = %d, want 2 (two distinct artists sharing a name): %+v", len(got), got)
	}
	a := findArtist(t, got, "mbid-a")
	b := findArtist(t, got, "mbid-b")
	if !a.LabelCollision || !b.LabelCollision {
		t.Errorf("both groups sharing the label %q should be flagged LabelCollision: a=%+v b=%+v", "Overlap", a, b)
	}
	if a.NameMismatch || b.NameMismatch {
		t.Errorf("neither group has an internal name disagreement, NameMismatch should be false: a=%+v b=%+v", a, b)
	}
}

func TestGroupArtists_SameMBIDDifferentNameStringsStayMerged(t *testing.T) {
	rows := []ArtistRow{
		{Artist: "The Artist", ArtistMBID: "mbid-x"},
		{Artist: "Artist, The", ArtistMBID: "mbid-x"},
		{Artist: "The Artist", ArtistMBID: "mbid-x"},
	}
	got := GroupArtists(rows)
	if len(got) != 1 {
		t.Fatalf("len(got) = %d, want 1 (one artist, inconsistent name strings): %+v", len(got), got)
	}
	g := got[0]
	if g.Key != "mbid-x" {
		t.Errorf("Key = %q, want mbid-x", g.Key)
	}
	if !g.NameMismatch {
		t.Errorf("NameMismatch should be true: %+v", g)
	}
	if g.Artist != "The Artist" {
		t.Errorf("representative label = %q, want the most-frequent name %q", g.Artist, "The Artist")
	}
	if len(g.DistinctNames) != 2 {
		t.Errorf("DistinctNames = %+v, want 2 distinct names", g.DistinctNames)
	}
	if g.TrackCount != 3 {
		t.Errorf("TrackCount = %d, want 3", g.TrackCount)
	}
}

func TestGroupArtists_UnidentifiedGroupsNeverGetNameMismatch(t *testing.T) {
	rows := []ArtistRow{
		{Artist: "", RawArtist: "Raw Only", ArtistMBID: ""},
		{Artist: "", RawArtist: "Raw Only", ArtistMBID: ""},
	}
	got := GroupArtists(rows)
	if len(got) != 1 {
		t.Fatalf("len(got) = %d, want 1: %+v", len(got), got)
	}
	if got[0].NameMismatch {
		t.Errorf("a name-derived (unidentified) group must never report NameMismatch: %+v", got[0])
	}
	if got[0].Key != "name:Raw Only" {
		t.Errorf("Key = %q, want name-derived key", got[0].Key)
	}
}

func TestGroupArtists_NoFalsePositivesForACleanGroup(t *testing.T) {
	rows := []ArtistRow{
		{Artist: "Clean Artist", ArtistMBID: "mbid-clean"},
		{Artist: "Clean Artist", ArtistMBID: "mbid-clean"},
		{Artist: "", RawArtist: "Unrelated", ArtistMBID: ""},
	}
	got := GroupArtists(rows)
	clean := findArtist(t, got, "mbid-clean")
	if clean.NameMismatch || clean.LabelCollision {
		t.Errorf("a clean, non-colliding group should have no flags set: %+v", clean)
	}
}

func TestGroupArtists_UnknownBucketForFilesWithNoMetadata(t *testing.T) {
	rows := []ArtistRow{{}}
	got := GroupArtists(rows)
	if len(got) != 1 {
		t.Fatalf("len(got) = %d, want 1", len(got))
	}
	if got[0].Artist != UnknownArtist {
		t.Errorf("Artist = %q, want %q", got[0].Artist, UnknownArtist)
	}
}

func TestGroupAlbums_ScopedGroupingSameAsArtists(t *testing.T) {
	rows := []AlbumRow{
		{Album: "Reissue", ReleaseGroupMBID: "rg-1"},
		{Album: "Original Title", ReleaseGroupMBID: "rg-1"},
		{Album: "Other Album", ReleaseGroupMBID: "rg-2"},
	}
	got := GroupAlbums(rows)
	if len(got) != 2 {
		t.Fatalf("len(got) = %d, want 2: %+v", len(got), got)
	}
	var rg1 AlbumSummary
	for _, a := range got {
		if a.Key == "rg-1" {
			rg1 = a
		}
	}
	if !rg1.NameMismatch {
		t.Errorf("rg-1 group should report NameMismatch: %+v", rg1)
	}
}
