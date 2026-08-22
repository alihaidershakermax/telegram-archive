package handlers

import "testing"

func TestParseSearchQueryTypeAndDownloadsSort(t *testing.T) {
	query, params := parseSearchQuery("تشريح type:pdf sort:downloads")
	if query != "تشريح" {
		t.Fatalf("query = %q, want %q", query, "تشريح")
	}
	if params.FileType != "pdf" {
		t.Fatalf("file type = %q, want %q", params.FileType, "pdf")
	}
	if params.Sort != "downloads" {
		t.Fatalf("sort = %q, want %q", params.Sort, "downloads")
	}
}

func TestParseSearchQueryArabicFilters(t *testing.T) {
	query, params := parseSearchQuery("فيزياء النوع:pdf الترتيب:الأكثر_تنزيلاً")
	if query != "فيزياء" {
		t.Fatalf("query = %q, want %q", query, "فيزياء")
	}
	if params.FileType != "pdf" || params.Sort != "downloads" {
		t.Fatalf("unexpected params: file_type=%q sort=%q", params.FileType, params.Sort)
	}
}

func TestParseSearchQueryIgnoresUnknownSort(t *testing.T) {
	query, params := parseSearchQuery("biology sort:unknown")
	if query != "biology" {
		t.Fatalf("query = %q, want %q", query, "biology")
	}
	if params.Sort != "newest" {
		t.Fatalf("sort = %q, want newest", params.Sort)
	}
}

// Keep the search parser contract explicit for the API-compatible filter syntax.
func TestParseSearchQueryPreservesPhraseWords(t *testing.T) {
	query, _ := parseSearchQuery("linear algebra type:pdf")
	if query != "linear algebra" {
		t.Fatalf("query = %q, want %q", query, "linear algebra")
	}
}

// This test documents the exact user-facing examples used in the command help.
func TestParseSearchQueryCommandExample(t *testing.T) {
	query, params := parseSearchQuery("تشريح النوع:pdf الترتيب:الأكثر_تنزيلاً")
	if query != "تشريح" || params.FileType != "pdf" || params.Sort != "downloads" {
		t.Fatalf("unexpected parsed command: query=%q type=%q sort=%q", query, params.FileType, params.Sort)
	}
}
