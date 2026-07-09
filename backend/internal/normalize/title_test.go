package normalize

import "testing"

func TestNormalizeTitle(t *testing.T) {
    cases := []struct {
        in, want string
    }{
        {"Ubuntu 22.04", "ubuntu 22 04"},
        {"Foo_Bar-Baz", "foo bar baz"},
        {"  Multiple   Spaces  ", "multiple spaces"},
        {"Café", "café"}, // NFKC keeps accented chars but case-folds
        {"Héllo Wörld", "héllo wörld"},
        {"Trailing-Punctuation---", "trailing punctuation"},
        {"", ""},
        {"ABC.123_def-456", "abc 123 def 456"},
    }
    for _, tc := range cases {
        t.Run(tc.in, func(t *testing.T) {
            if got := NormalizeTitle(tc.in); got != tc.want {
                t.Errorf("NormalizeTitle(%q) = %q, want %q", tc.in, got, tc.want)
            }
        })
    }
}

func TestNormalizeTitleStableForEquivalentForms(t *testing.T) {
    // Same logical title in different surface forms should dedup.
    if NormalizeTitle("Ubuntu.22.04.LTS") != NormalizeTitle("ubuntu-22-04_lts") {
        t.Fatal("equivalent forms should produce equal keys")
    }
}
