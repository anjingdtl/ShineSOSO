package model

import "time"

// ResultSource is one indexer's contribution to a (possibly merged) result.
// A result with multiple sources has been deduped (spec §17.3).
type ResultSource struct {
    IndexerID   string     `json:"indexerId"`
    IndexerName string     `json:"indexerName"`
    MagnetURL   string     `json:"magnetUrl,omitempty"`
    TorrentURL  string     `json:"torrentUrl,omitempty"`
    DirectURL   string     `json:"directUrl,omitempty"`
    DetailURL   string     `json:"detailUrl,omitempty"`
    Seeders     *int       `json:"seeders,omitempty"`
    PublishedAt *time.Time `json:"publishedAt,omitempty"`
}

// SearchResult is the normalized, possibly-merged result returned to the UI.
// All times are UTC. Sizes are bytes. Optional fields use pointers so the
// UI can distinguish "absent" from "zero".
type SearchResult struct {
    ID             string         `json:"id"`
    Title          string         `json:"title"`
    NormalizedTitle string        `json:"-"`
    Category       string         `json:"category"`
    SizeBytes      *int64         `json:"sizeBytes,omitempty"`
    Seeders        *int           `json:"seeders,omitempty"`
    Leechers       *int           `json:"leechers,omitempty"`
    Downloads      *int           `json:"downloads,omitempty"`
    PublishedAt    *time.Time     `json:"publishedAt,omitempty"`
    MagnetURL      string         `json:"magnetUrl,omitempty"`
    TorrentURL     string         `json:"torrentUrl,omitempty"`
    DirectURL      string         `json:"directUrl,omitempty"`
    DetailURL      string         `json:"detailUrl,omitempty"`
    InfoHash       string         `json:"infoHash,omitempty"`
    IndexerID      string         `json:"indexerId"`
    IndexerName    string         `json:"indexerName"`
    Score          float64        `json:"score,omitempty"`
    Sources        []ResultSource `json:"sources,omitempty"`
}

// PrimaryDownload returns the highest-priority download URL the result
// can offer, in order magnet > torrent > direct > detail (spec §6.8).
func (r *SearchResult) PrimaryDownload() (url, kind string) {
    switch {
    case r.MagnetURL != "":
        return r.MagnetURL, "magnet"
    case r.TorrentURL != "":
        return r.TorrentURL, "torrent"
    case r.DirectURL != "":
        return r.DirectURL, "direct"
    case r.DetailURL != "":
        return r.DetailURL, "detail"
    default:
        return "", ""
    }
}
