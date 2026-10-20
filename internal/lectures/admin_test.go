package lectures

import (
	"strings"
	"testing"
)

func TestValidSlug(t *testing.T) {
	for _, s := range []string{"a", "lesson-1", "intro-to-go-2"} {
		if !ValidSlug(s) {
			t.Errorf("ValidSlug(%q) = false, want true", s)
		}
	}
	for _, s := range []string{"", "-a", "a-", "a--b", "Lesson", "a_b", "a b", "урок"} {
		if ValidSlug(s) {
			t.Errorf("ValidSlug(%q) = true, want false", s)
		}
	}
}

func TestIsPDF(t *testing.T) {
	cases := map[string]bool{
		"%PDF-1.7\n...": true,
		"%PDF-":         true,
		"":              false,
		"%PDF":          false,
		"<html>":        false,
		" %PDF-1.4":     false,
	}
	for in, want := range cases {
		if got := IsPDF([]byte(in)); got != want {
			t.Errorf("IsPDF(%q) = %v, want %v", in, got, want)
		}
	}
}

func TestInputNormalize(t *testing.T) {
	blank := "  "
	in := Input{Slug: " lesson-1 ", Title: " Intro ", Description: &blank}
	if !in.Normalize() {
		t.Fatal("Normalize = false, want true")
	}
	if in.Slug != "lesson-1" || in.Title != "Intro" || in.Description != nil || in.Active == nil || !*in.Active {
		t.Fatalf("Normalize result = %+v", in)
	}
	if (&Input{Slug: "lesson-1", Title: "  "}).Normalize() {
		t.Fatal("empty title should be invalid")
	}
}

func TestNormalizeVideoID(t *testing.T) {
	long := strings.Repeat("a", 1024)
	cases := []struct {
		in, want string
		ok       bool
	}{
		{"  blob/video.mp4 ", "blob/video.mp4", true},
		{long, long, true},
		{strings.Repeat("я", 1024), strings.Repeat("я", 1024), true},
		{long + "a", "", false},
		{"", "", false},
		{"   ", "", false},
	}
	for _, tc := range cases {
		got, ok := NormalizeVideoID(tc.in)
		if ok != tc.ok || (ok && got != tc.want) {
			t.Errorf("NormalizeVideoID(%.20q) = %.20q, %v; want %.20q, %v", tc.in, got, ok, tc.want, tc.ok)
		}
	}
}
