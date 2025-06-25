// Package pages provides HTML pages for the admin UI.
package pages

import (
	"bytes"
	"encoding/json"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"time"

	"github.com/dustin/go-humanize"
)

// PageData holds common data for all pages
type PageData struct {
	Title       string
	Description string
	Time        string
	AppName     string
	Version     string
	NavLinks    []NavLink
	Content     interface{}
}

// NavLink represents a navigation link
type NavLink struct {
	URL      string
	Name     string
	Active   bool
	Icon     string
	External bool
}

// Common navigation links used across pages
var commonNavLinks = []NavLink{
	{URL: "/", Name: "Dashboard", Icon: "home"},
	{URL: "/databases", Name: "Database Browser", Icon: "database"},
	{URL: "/status/backups", Name: "Backup Status", Icon: "list"},
	{URL: "/status/storage", Name: "Storage", Icon: "hard-drive"},
	{URL: "/servers", Name: "Servers", Icon: "server"},
	{URL: "/mysql-options", Name: "MySQL Options", Icon: "settings"},
	{URL: "/metrics", Name: "Metrics", Icon: "bar-chart-2", External: true},
}

// formatDuration formats a duration for display
func formatDuration(d time.Duration) string {
	d = d.Round(time.Second)
	if d < time.Minute {
		return fmt.Sprintf("%d seconds", int(d.Seconds()))
	}
	if d < time.Hour {
		minutes := int(d.Minutes())
		seconds := int(d.Seconds()) % 60
		return fmt.Sprintf("%d minutes, %d seconds", minutes, seconds)
	}
	hours := int(d.Hours())
	minutes := int(d.Minutes()) % 60
	return fmt.Sprintf("%d hours, %d minutes", hours, minutes)
}

// generateCommonTemplate creates the base template with common elements
func generateCommonTemplate() *template.Template {
	// Define common functions
	funcs := template.FuncMap{
		"formatTime": func(t time.Time) string {
			return t.Format("2006-01-02 15:04:05")
		},
		"json": func(v interface{}) template.JS {
			// Convert the value to JSON for safe embedding in JavaScript
			b, err := json.Marshal(v)
			if err != nil {
				return template.JS("null")
			}
			return template.JS(b)
		},
		"formatBytes": func(v interface{}) string {
			switch val := v.(type) {
			case uint64:
				return humanize.Bytes(val)
			case int64:
				return humanize.Bytes(uint64(val))
			case int:
				return humanize.Bytes(uint64(val))
			case float64:
				return humanize.Bytes(uint64(val))
			default:
				return "0 B"
			}
		},
		"formatDuration": formatDuration,
		"timeAgo":        humanize.Time,
		"safeHTML": func(s string) template.HTML {
			return template.HTML(s)
		},
	}

	// Base template with common layout
	baseTemplate := `
<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>{{ .Title }} - {{ .AppName }}</title>
    <meta name="description" content="{{ .Description }}">
    <link rel="stylesheet" href="https://cdn.jsdelivr.net/npm/bootstrap@5.3.0/dist/css/bootstrap.min.css">
    <link rel="stylesheet" href="https://cdn.jsdelivr.net/npm/feather-icons/dist/feather.min.css">
    <style>
        body {
            padding-top: 20px;
            font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, "Helvetica Neue", Arial, sans-serif;
        }
        .navbar {
            margin-bottom: 20px;
        }
        .nav-link .feather {
            width: 16px;
            height: 16px;
            margin-right: 4px;
        }
        .card {
            margin-bottom: 20px;
        }
        .bg-success-light {
            background-color: rgba(40, 167, 69, 0.1);
        }
        .bg-danger-light {
            background-color: rgba(220, 53, 69, 0.1);
        }
        .bg-info-light {
            background-color: rgba(23, 162, 184, 0.1);
        }
        .bg-warning-light {
            background-color: rgba(255, 193, 7, 0.1);
        }
        .status-badge {
            font-size: 0.8rem;
            padding: 0.25rem 0.5rem;
        }
        footer {
            margin-top: 3rem;
            padding: 1.5rem 0;
            border-top: 1px solid #e9ecef;
            color: #6c757d;
            font-size: 0.9rem;
        }
    </style>
</head>
<body>
    <div class="container">
        <header class="pb-3 mb-4 border-bottom">
            <a href="/" class="d-flex align-items-center text-dark text-decoration-none">
                <span class="fs-4">{{ .AppName }}</span>
            </a>
        </header>

        <nav class="navbar navbar-expand-lg navbar-light bg-light rounded">
            <div class="container-fluid">
                <button class="navbar-toggler" type="button" data-bs-toggle="collapse" data-bs-target="#navbarNavAltMarkup">
                    <span class="navbar-toggler-icon"></span>
                </button>
                <div class="collapse navbar-collapse" id="navbarNavAltMarkup">
                    <div class="navbar-nav">
                        {{ range .NavLinks }}
                        <a class="nav-link {{ if .Active }}active{{ end }} {{ if .External }}text-primary{{ end }}" href="{{ .URL }}">
                            <i data-feather="{{ .Icon }}"></i> {{ .Name }}
                        </a>
                        {{ end }}
                    </div>
                </div>
            </div>
        </nav>

        <main class="mt-4">
            <h1>{{ .Title }}</h1>
            <p class="lead">{{ .Description }}</p>
            
            {{ block "content" . }}{{ end }}
        </main>

        <footer class="text-center">
            <div>{{ .AppName }} {{ .Version }}</div>
            <div class="text-muted">Page rendered at {{ .Time }}</div>
        </footer>
    </div>

    <script src="https://cdn.jsdelivr.net/npm/bootstrap@5.3.0/dist/js/bootstrap.bundle.min.js"></script>
    <script src="https://cdn.jsdelivr.net/npm/feather-icons/dist/feather.min.js"></script>
    <script>
        document.addEventListener('DOMContentLoaded', () => {
            feather.replace();
        });
    </script>
</body>
</html>
`

	// Parse the base template
	tmpl, err := template.New("base").Funcs(funcs).Parse(baseTemplate)
	if err != nil {
		log.Printf("Error parsing template: %v", err)
		return nil
	}

	return tmpl
}

// renderTemplate renders a template with the provided data
func renderTemplate(w http.ResponseWriter, tmpl *template.Template, name string, data PageData) {
	// Set default values if not provided
	if data.AppName == "" {
		data.AppName = "GoSQLGuard"
	}
	if data.Version == "" {
		data.Version = "1.0"
	}
	if data.Time == "" {
		data.Time = time.Now().Format("2006-01-02 15:04:05")
	}
	if len(data.NavLinks) == 0 {
		data.NavLinks = commonNavLinks
	}

	// Mark the active nav link
	for i := range data.NavLinks {
		if data.NavLinks[i].URL == name {
			data.NavLinks[i].Active = true
		}
	}

	// Render the template
	buf := &bytes.Buffer{}
	err := tmpl.ExecuteTemplate(buf, "base", data)
	if err != nil {
		http.Error(w, fmt.Sprintf("Template error: %v", err), http.StatusInternalServerError)
		log.Printf("Template error: %v", err)
		return
	}

	// Send the result
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	buf.WriteTo(w)
}
