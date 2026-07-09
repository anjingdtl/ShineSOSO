package launcher

import (
    "os"
    "path/filepath"
    "testing"
)

func writeRaw(path, content string) error {
    return os.WriteFile(path, []byte(content), 0o644)
}

func TestWriteReadPortFile(t *testing.T) {
    dir := t.TempDir()
    path, err := WritePortFile(dir, 18765)
    if err != nil {
        t.Fatal(err)
    }
    if path != filepath.Join(dir, PortFileName) {
        t.Fatalf("port file path wrong: %s", path)
    }
    got, err := ReadPortFile(dir)
    if err != nil {
        t.Fatal(err)
    }
    if got != 18765 {
        t.Fatalf("port want 18765 got %d", got)
    }
}

func TestReadPortFileMissing(t *testing.T) {
    dir := t.TempDir()
    if _, err := ReadPortFile(dir); err == nil {
        t.Fatal("expected error for missing port file")
    }
}

func TestReadPortFileMalformed(t *testing.T) {
    dir := t.TempDir()
    if _, err := WritePortFile(dir, 0); err != nil {
        t.Fatal(err)
    }
    // overwrite with garbage
    path := filepath.Join(dir, PortFileName)
    if err := writeRaw(path, "not-a-number"); err != nil {
        t.Fatal(err)
    }
    if _, err := ReadPortFile(dir); err == nil {
        t.Fatal("expected parse error for malformed port file")
    }
}

func TestWritePortFileRejectsEmptyDir(t *testing.T) {
    if _, err := WritePortFile("", 1234); err == nil {
        t.Fatal("expected error for empty dataDir")
    }
}

func TestRemovePortFile(t *testing.T) {
    dir := t.TempDir()
    if _, err := WritePortFile(dir, 1234); err != nil {
        t.Fatal(err)
    }
    if err := RemovePortFile(dir); err != nil {
        t.Fatal(err)
    }
    if _, err := ReadPortFile(dir); err == nil {
        t.Fatal("expected missing-file error after remove")
    }
    // second remove is a no-op error, which is fine
    _ = RemovePortFile(dir)
}
