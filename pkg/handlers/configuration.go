package handlers

import (
	"context"
	"net/http"
	"time"

	"github.com/supporttools/GoSQLGuard/pkg/config"
	"github.com/supporttools/GoSQLGuard/templates/pages"
	"github.com/supporttools/GoSQLGuard/templates/types"
)

// ConfigurationHandler handles the configuration management UI page
func ConfigurationHandler(w http.ResponseWriter, r *http.Request) {
	// Always using MySQL configuration now
	isYAMLConfig := false

	// Prepare configuration data
	configData := pages.ConfigurationPageData{
		IsYAMLConfig: isYAMLConfig,
		Config:       &config.CFG,
	}

	// Prepare page data
	pageData := types.PageData{
		Title:       "Configuration Management",
		Description: "Manage GoSQLGuard configuration settings",
		AppName:     "GoSQLGuard",
		Version:     "1.0",
		Time:        time.Now().Format("2006-01-02 15:04:05"),
		NavLinks:    getNavLinksWithActive("/configuration"),
	}

	// Render using Templ
	component := pages.ConfigurationPage(pageData, configData)
	component.Render(context.Background(), w)
}

// getNavLinksWithActive returns navigation links with the specified path marked as active
func getNavLinksWithActive(activePath string) []types.NavLink {
	links := []types.NavLink{
		{URL: "/", Name: "Dashboard", Icon: "home"},
		{URL: "/databases", Name: "Database Browser", Icon: "database"},
		{URL: "/status/backups", Name: "Backup Status", Icon: "list"},
		{URL: "/status/storage", Name: "Storage", Icon: "hard-drive"},
		{URL: "/servers", Name: "Servers", Icon: "server"},
		{URL: "/configuration", Name: "Configuration", Icon: "settings"},
		{URL: "/mysql-options", Name: "MySQL Options", Icon: "tool"},
		{URL: "/metrics", Name: "Metrics", Icon: "bar-chart-2", External: true},
	}

	for i := range links {
		if links[i].URL == activePath {
			links[i].Active = true
		}
	}

	return links
}