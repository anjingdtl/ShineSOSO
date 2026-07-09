package model

import "testing"

func TestPrimaryDownloadPriority(t *testing.T) {
    cases := []struct {
        name     string
        result   SearchResult
        wantURL  string
        wantKind string
    }{
        {
            name:     "magnet wins over all",
            result:   SearchResult{MagnetURL: "magnet:?xt=abc", TorrentURL: "https://x/t", DirectURL: "https://d", DetailURL: "https://p"},
            wantURL:  "magnet:?xt=abc",
            wantKind: "magnet",
        },
        {
            name:     "torrent beats direct and detail",
            result:   SearchResult{TorrentURL: "https://x/t", DirectURL: "https://d", DetailURL: "https://p"},
            wantURL:  "https://x/t",
            wantKind: "torrent",
        },
        {
            name:     "direct beats detail",
            result:   SearchResult{DirectURL: "https://d", DetailURL: "https://p"},
            wantURL:  "https://d",
            wantKind: "direct",
        },
        {
            name:     "only detail is allowed",
            result:   SearchResult{DetailURL: "https://p"},
            wantURL:  "https://p",
            wantKind: "detail",
        },
        {
            name:     "empty when nothing present",
            result:   SearchResult{},
            wantURL:  "",
            wantKind: "",
        },
    }
    for _, tc := range cases {
        t.Run(tc.name, func(t *testing.T) {
            url, kind := tc.result.PrimaryDownload()
            if url != tc.wantURL || kind != tc.wantKind {
                t.Fatalf("got (%q, %q), want (%q, %q)", url, kind, tc.wantURL, tc.wantKind)
            }
        })
    }
}
