package frappe

import (
	"errors"
	"fmt"
	"log/slog"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"gopkg.in/yaml.v3"
)

// ErrUnknownSite is returned when a requested site is not recognized by routes mapping.
type ErrUnknownSite struct {
	Site string
}

func (e ErrUnknownSite) Error() string {
	return fmt.Sprintf("unknown site: %s", e.Site)
}

// SiteResolver resolves a tenant site name to its Frappe bench base URL.
type SiteResolver struct {
	defaultURL     string
	routesFile     string
	urlTemplate    string
	reloadInterval time.Duration
	routes         map[string]string
	lastMtime      time.Time
	lastCheck      time.Time
	mu             sync.RWMutex
}

// NewSiteResolver creates a new SiteResolver with optional routes file and URL template.
func NewSiteResolver(defaultURL, routesFile, urlTemplate string) (*SiteResolver, error) {
	if defaultURL != "" {
		if _, err := url.ParseRequestURI(defaultURL); err != nil {
			return nil, fmt.Errorf("invalid default_url: %w", err)
		}
	}
	if urlTemplate != "" && !strings.Contains(urlTemplate, "{site}") {
		return nil, errors.New("url_template must contain '{site}' placeholder")
	}

	sr := &SiteResolver{
		defaultURL:     strings.TrimSuffix(defaultURL, "/"),
		routesFile:     routesFile,
		urlTemplate:    urlTemplate,
		reloadInterval: 2 * time.Second,
		routes:         make(map[string]string),
	}

	if routesFile != "" {
		if err := sr.loadRoutes(true); err != nil {
			return nil, fmt.Errorf("failed to load site routes file: %w", err)
		}
	}

	return sr, nil
}

func (sr *SiteResolver) loadRoutes(force bool) error {
	if sr.routesFile == "" {
		return nil
	}

	now := time.Now()
	if !force && now.Sub(sr.lastCheck) < sr.reloadInterval {
		return nil
	}
	sr.lastCheck = now

	cleanedPath := filepath.Clean(sr.routesFile)
	fi, err := os.Stat(cleanedPath)
	if err != nil {
		if force {
			return err
		}
		slog.Warn("site routes file stat failed", "file", cleanedPath, "error", err)
		return nil
	}

	if !force && fi.ModTime().Equal(sr.lastMtime) {
		return nil
	}

	data, err := os.ReadFile(cleanedPath)
	if err != nil {
		if force {
			return err
		}
		slog.Warn("failed to read site routes file", "file", cleanedPath, "error", err)
		return nil
	}

	var parsed struct {
		Routes map[string]string `yaml:"routes"`
	}
	if err := yaml.Unmarshal(data, &parsed); err != nil {
		if force {
			return fmt.Errorf("failed to parse routes YAML: %w", err)
		}
		slog.Warn("failed to parse site routes YAML during reload", "file", cleanedPath, "error", err)
		return nil
	}

	normalized := make(map[string]string, len(parsed.Routes))
	for site, u := range parsed.Routes {
		site = strings.TrimSpace(site)
		u = strings.TrimSpace(u)
		if site == "" || u == "" {
			continue
		}
		normalized[site] = strings.TrimSuffix(u, "/")
	}

	sr.routes = normalized
	sr.lastMtime = fi.ModTime()
	slog.Info("site routes loaded", "count", len(normalized), "file", cleanedPath)
	return nil
}

// Resolve returns the Frappe bench base URL for the given tenant site.
func (sr *SiteResolver) Resolve(site string) (string, error) {
	site = strings.TrimSpace(site)
	if sr.routesFile != "" && site == "" {
		return "", ErrUnknownSite{Site: ""}
	}
	if site == "" {
		return sr.defaultURL, nil
	}

	if sr.routesFile != "" {
		sr.mu.Lock()
		_ = sr.loadRoutes(false)
		u, ok := sr.routes[site]
		sr.mu.Unlock()
		if ok {
			return u, nil
		}
		return "", ErrUnknownSite{Site: site}
	}

	if sr.urlTemplate != "" {
		formatted := strings.ReplaceAll(sr.urlTemplate, "{site}", site)
		return strings.TrimSuffix(formatted, "/"), nil
	}

	return sr.defaultURL, nil
}
