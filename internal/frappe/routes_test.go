package frappe

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestSiteResolverDefaultFallback(t *testing.T) {
	sr, err := NewSiteResolver("http://bench-default:8000", "", "")
	if err != nil {
		t.Fatalf("NewSiteResolver: %v", err)
	}

	for _, site := range []string{"", "tenant1.local", "random.site"} {
		got, err := sr.Resolve(site)
		if err != nil {
			t.Errorf("Resolve(%q) returned unexpected error: %v", site, err)
		}
		if got != "http://bench-default:8000" {
			t.Errorf("Resolve(%q) = %q, want http://bench-default:8000", site, got)
		}
	}
}

func TestSiteResolverTemplate(t *testing.T) {
	sr, err := NewSiteResolver("http://bench-default:8000", "", "http://{site}.svc.cluster.local:8000")
	if err != nil {
		t.Fatalf("NewSiteResolver: %v", err)
	}

	got, err := sr.Resolve("tenant-a")
	if err != nil {
		t.Fatalf("Resolve(tenant-a): %v", err)
	}
	if got != "http://tenant-a.svc.cluster.local:8000" {
		t.Errorf("got %q, want http://tenant-a.svc.cluster.local:8000", got)
	}

	// Empty site falls back to default
	gotEmpty, err := sr.Resolve("")
	if err != nil {
		t.Fatalf("Resolve(''): %v", err)
	}
	if gotEmpty != "http://bench-default:8000" {
		t.Errorf("got %q, want http://bench-default:8000", gotEmpty)
	}
}

func TestSiteResolverRoutesFileHotReload(t *testing.T) {
	tmpDir := t.TempDir()
	routesPath := filepath.Join(tmpDir, "routes.yaml")

	initialYAML := `routes:
  tenant1.local: http://bench-01:8000
  tenant2.local: http://bench-02:8000
`
	if err := os.WriteFile(routesPath, []byte(initialYAML), 0600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	sr, err := NewSiteResolver("http://bench-default:8000", routesPath, "")
	if err != nil {
		t.Fatalf("NewSiteResolver: %v", err)
	}
	sr.reloadInterval = 10 * time.Millisecond

	// Mapped sites resolve
	got1, err := sr.Resolve("tenant1.local")
	if err != nil || got1 != "http://bench-01:8000" {
		t.Errorf("Resolve(tenant1.local) = %q, %v", got1, err)
	}
	got2, err := sr.Resolve("tenant2.local")
	if err != nil || got2 != "http://bench-02:8000" {
		t.Errorf("Resolve(tenant2.local) = %q, %v", got2, err)
	}

	// Unknown site fails
	_, err = sr.Resolve("unknown.local")
	if err == nil {
		t.Errorf("Resolve(unknown.local) expected ErrUnknownSite, got nil")
	}

	// Missing/empty site with routes configured fails
	_, err = sr.Resolve("")
	if err == nil {
		t.Errorf("Resolve('') with routes file expected ErrUnknownSite, got nil")
	}

	// Modify file and hot reload
	time.Sleep(20 * time.Millisecond)
	updatedYAML := `routes:
  tenant1.local: http://bench-01:8000
  unknown.local: http://bench-03:8000
`
	if err := os.WriteFile(routesPath, []byte(updatedYAML), 0600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	gotNew, err := sr.Resolve("unknown.local")
	if err != nil || gotNew != "http://bench-03:8000" {
		t.Errorf("After reload: Resolve(unknown.local) = %q, %v, want http://bench-03:8000", gotNew, err)
	}
}
