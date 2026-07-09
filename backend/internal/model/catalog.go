package model

import "time"

// CatalogEntry is one item in the built-in or imported indexer catalog
// (spec §26.2 manifest). Used by GET /api/v1/indexer-catalog.
type CatalogEntry struct {
    ID              string    `json:"id"`
    Name            string    `json:"name"`
    Description     string    `json:"description,omitempty"`
    Version         string    `json:"version"`
    Language        string    `json:"language,omitempty"`
    Protocol        string    `json:"protocol"`
    Categories      []string  `json:"categories,omitempty"`
    Homepage        string    `json:"homepage,omitempty"`
    Installed       bool      `json:"installed"`
    InstalledID     string    `json:"installedId,omitempty"`
    InstalledStatus string    `json:"installedStatus,omitempty"`
    Definition      IndexerDefinition `json:"definition"`
    UpdatedAt       time.Time `json:"updatedAt,omitempty"`
}

// CatalogManifest is the top-level manifest.json of the indexer catalog
// (spec §26.2). Used by the updater to know which yml files to fetch.
type CatalogManifest struct {
    Schema      int                       `json:"schema"`
    Version     string                    `json:"version"`
    GeneratedAt time.Time                 `json:"generatedAt"`
    Definitions []CatalogManifestEntry    `json:"definitions"`
}

type CatalogManifestEntry struct {
    ID      string `json:"id"`
    Version string `json:"version"`
    File    string `json:"file"`
    SHA256  string `json:"sha256"`
}

// ImportedDefinition is a user-imported YAML indexer definition, stored
// verbatim in SQLite (spec §20.4). Future reloads compare Checksum.
type ImportedDefinition struct {
    ID        string    `json:"id"`
    Version   string    `json:"version"`
    Content   string    `json:"content"`
    Checksum  string    `json:"checksum"`
    CreatedAt time.Time `json:"createdAt"`
    UpdatedAt time.Time `json:"updatedAt"`
}
