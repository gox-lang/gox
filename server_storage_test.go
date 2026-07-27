package gox

import (
	"strings"
	"testing"
)

type serverStorageTestProvider struct {
	Name string
}

type serverStorageTestInterface interface {
	ProviderName() string
}

type serverStorageInterfaceProvider struct{}

func (serverStorageInterfaceProvider) ProviderName() string { return "provider" }

func TestServerStorageSetGetHasDelete(t *testing.T) {
	key := "test.server.storage.provider"
	Storage.Delete(key)

	if Storage.Has(key) {
		t.Fatalf("storage should not have %q before Set", key)
	}

	provider := &serverStorageTestProvider{Name: "mongo"}
	Storage.Set(key, provider)

	if !Storage.Has(key) {
		t.Fatalf("storage should have %q after Set", key)
	}

	var got *serverStorageTestProvider
	ok, err := Storage.Get(key, &got)
	if err != nil {
		t.Fatalf("Get returned error: %v", err)
	}
	if !ok {
		t.Fatalf("Get returned ok=false")
	}
	if got != provider {
		t.Fatalf("Get returned %#v, want original provider %#v", got, provider)
	}

	Storage.Delete(key)
	if Storage.Has(key) {
		t.Fatalf("storage should not have %q after Delete", key)
	}
}

func TestServerStorageGetMissing(t *testing.T) {
	key := "test.server.storage.missing"
	Storage.Delete(key)

	var got *serverStorageTestProvider
	ok, err := Storage.Get(key, &got)
	if err != nil {
		t.Fatalf("Get missing returned error: %v", err)
	}
	if ok {
		t.Fatalf("Get missing returned ok=true")
	}
	if got != nil {
		t.Fatalf("Get missing changed target: %#v", got)
	}
}

func TestServerStorageGetTypeMismatch(t *testing.T) {
	key := "test.server.storage.type_mismatch"
	Storage.Set(key, "not-a-provider")
	defer Storage.Delete(key)

	var got *serverStorageTestProvider
	ok, err := Storage.Get(key, &got)
	if err == nil {
		t.Fatalf("Get type mismatch returned nil error")
	}
	if ok {
		t.Fatalf("Get type mismatch returned ok=true")
	}
	if !strings.Contains(err.Error(), "target expects") {
		t.Fatalf("Get type mismatch error = %q", err.Error())
	}
}

func TestServerStorageGetInvalidOut(t *testing.T) {
	key := "test.server.storage.invalid_out"
	Storage.Set(key, &serverStorageTestProvider{})
	defer Storage.Delete(key)

	var got *serverStorageTestProvider
	ok, err := Storage.Get(key, got)
	if err == nil {
		t.Fatalf("Get invalid out returned nil error")
	}
	if ok {
		t.Fatalf("Get invalid out returned ok=true")
	}
}

func TestServerStorageGetInterfaceTarget(t *testing.T) {
	key := "test.server.storage.interface"
	provider := serverStorageInterfaceProvider{}
	Storage.Set(key, provider)
	defer Storage.Delete(key)

	var got serverStorageTestInterface
	ok, err := Storage.Get(key, &got)
	if err != nil {
		t.Fatalf("Get interface target returned error: %v", err)
	}
	if !ok {
		t.Fatalf("Get interface target returned ok=false")
	}
	if got.ProviderName() != "provider" {
		t.Fatalf("Get interface target returned unexpected provider %q", got.ProviderName())
	}
}
