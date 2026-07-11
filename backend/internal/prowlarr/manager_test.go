package prowlarr

import "testing"

func TestSchemaIdentifierPrefersCardigannDefinitionName(t *testing.T) {
	schema := map[string]any{"definitionName": "nyaasi", "implementation": "Cardigann"}
	if got := schemaIdentifier(schema); got != "nyaasi" {
		t.Fatalf("schema id = %q, want nyaasi", got)
	}
}

func TestNeedsInputAllowsOptionalTuningButRejectsCredentials(t *testing.T) {
	public := map[string]any{"fields": []any{
		map[string]any{"name": "definitionFile", "type": "textbox", "value": "nyaasi"},
		map[string]any{"name": "baseUrl", "type": "select"},
		map[string]any{"name": "seedRatio", "type": "number"},
		map[string]any{"name": "baseSettings.queryLimit", "type": "number", "advanced": true},
	}}
	if needsInput(public) {
		t.Fatal("optional public settings must remain one-click eligible")
	}
	private := map[string]any{"fields": []any{map[string]any{"name": "username", "type": "textbox", "value": ""}}}
	if !needsInput(private) {
		t.Fatal("credential field must require setup")
	}
}

func TestProwlarrLocalDownloadURL(t *testing.T) {
	if !isProwlarrLocalURL("http://127.0.0.1:9696", "http://127.0.0.1:9696/api/v1/download?x=1") {
		t.Fatal("managed local download URL should be proxied")
	}
	if isProwlarrLocalURL("http://127.0.0.1:9696", "https://example.com/file.torrent") {
		t.Fatal("external download URL must not be proxied")
	}
}
