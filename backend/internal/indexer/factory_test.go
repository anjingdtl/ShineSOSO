package indexer

import (
    "context"
    "errors"
    "testing"

    "github.com/local/easysearch/backend/internal/model"
)

func TestRegistryRegisterAndGet(t *testing.T) {
    r := NewRegistry()
    factory := fakeFactory{name: "fake"}
    r.Register(Protocol("foo"), factory)
    got, err := r.Get(Protocol("foo"))
    if err != nil {
        t.Fatal(err)
    }
    if got.(fakeFactory).name != "fake" {
        t.Fatalf("wrong factory: %+v", got)
    }
}

func TestRegistryUnknownProtocol(t *testing.T) {
    r := NewRegistry()
    _, err := r.Get(Protocol("nope"))
    if err == nil {
        t.Fatal("expected error for unknown protocol")
    }
}

func TestRegistryReRegister(t *testing.T) {
    r := NewRegistry()
    r.Register(Protocol("foo"), fakeFactory{name: "first"})
    r.Register(Protocol("foo"), fakeFactory{name: "second"})
    got, _ := r.Get(Protocol("foo"))
    if got.(fakeFactory).name != "second" {
        t.Fatal("re-register should replace")
    }
}

type fakeFactory struct {
    name string
}

func (f fakeFactory) Create(_ model.IndexerDefinition, _ model.InstalledIndexer, _ *Client) (IndexerAdapter, error) {
    return nil, errors.New("not used in this test")
}

// fakeAdapter satisfies IndexerAdapter without doing anything real.
type fakeAdapter struct{}

func (a fakeAdapter) ID() string                                                          { return "fake" }
func (a fakeAdapter) Test(_ context.Context) TestResult                                   { return TestResult{OK: true} }
func (a fakeAdapter) Search(_ context.Context, _ model.SearchQuery) ([]model.SearchResult, error) {
    return nil, nil
}

var _ IndexerAdapter = fakeAdapter{}
