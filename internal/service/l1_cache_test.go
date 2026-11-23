package service

import (
	"testing"
	"time"
)

func TestL1CacheSetGetExpireDelete(t *testing.T) {
	c := newL1Cache(2)

	c.Set("k1", "https://example.com/1", time.Minute)
	if v, ok := c.Get("k1"); !ok || v != "https://example.com/1" {
		t.Fatalf("esperava hit para k1, obteve (%q, %v)", v, ok)
	}

	c.Delete("k1")
	if _, ok := c.Get("k1"); ok {
		t.Fatalf("esperava miss após Delete(k1)")
	}

	c.Set("exp", "https://example.com/exp", 10*time.Millisecond)
	time.Sleep(20 * time.Millisecond)
	if _, ok := c.Get("exp"); ok {
		t.Fatalf("esperava expiração da chave exp")
	}

	// Verifica limite máximo de entradas
	c.Set("a", "1", time.Minute)
	c.Set("b", "2", time.Minute)
	c.Set("c", "3", time.Minute)
	if len(c.items) > 2 {
		t.Fatalf("cache excedeu maxEntries: %d > 2", len(c.items))
	}
}
