package api

import (
	"encoding/json"
	"log"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/supporttools/GoSQLGuard/pkg/config"
	dbmeta "github.com/supporttools/GoSQLGuard/pkg/database/metadata"
	"github.com/supporttools/GoSQLGuard/pkg/metadata"
	"github.com/supporttools/GoSQLGuard/pkg/scheduler"
)

// ScheduleHandler handles schedule management API endpoints
type ScheduleHandler struct {
	scheduleRepo *dbmeta.ScheduleRepository
	scheduler    *scheduler.Scheduler
}

// NewScheduleHandler creates a new schedule handler
func NewScheduleHandler(sched *scheduler.Scheduler) *ScheduleHandler {
	if metadata.DB == nil {
		log.Println("Warning: Database is not initialized, schedule management API will not work")
		return &ScheduleHandler{scheduler: sched}
	}

	return &ScheduleHandler{
		scheduleRepo: dbmeta.NewScheduleRepository(metadata.DB),
		scheduler:    sched,
	}
}

// RegisterRoutes registers the schedule API routes on the provided mux
func (h *ScheduleHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/api/schedules", h.handleSchedules)
	mux.HandleFunc("/api/schedules/delete", h.handleDeleteSchedule)
}

// handleSchedules handles GET and POST requests for schedule management
func (h *ScheduleHandler) handleSchedules(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		if h.scheduleRepo == nil {
			http.Error(w, "Schedule management is not available: database not initialized", http.StatusServiceUnavailable)
			return
		}
		h.getSchedules(w, r)
	case http.MethodPost:
		if h.scheduleRepo == nil {
			http.Error(w, "Schedule management is not available: database not initialized", http.StatusServiceUnavailable)
			return
		}
		h.createOrUpdateSchedule(w, r)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// getSchedules returns all schedules or a specific schedule
func (h *ScheduleHandler) getSchedules(w http.ResponseWriter, r *http.Request) {
	// Check if a specific schedule is requested
	scheduleID := r.URL.Query().Get("id")
	if scheduleID != "" {
		schedule, err := h.scheduleRepo.GetScheduleByID(scheduleID)
		if err != nil {
			http.Error(w, "Schedule not found: "+err.Error(), http.StatusNotFound)
			return
		}

		// Convert to response type
		response := convertScheduleToResponse(schedule)

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
		return
	}

	// Otherwise, return all schedules
	schedules, err := h.scheduleRepo.GetAllSchedules()
	if err != nil {
		http.Error(w, "Failed to retrieve schedules: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Convert to response type
	var response []scheduleResponse
	for _, schedule := range schedules {
		response = append(response, convertScheduleToResponse(&schedule))
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// scheduleRequest is the request structure for creating/updating a schedule
type scheduleRequest struct {
	ID             string         `json:"id,omitempty"`
	Name           string         `json:"name"`
	BackupType     string         `json:"backupType"`
	CronExpression string         `json:"cronExpression"`
	Enabled        bool           `json:"enabled"`
	LocalStorage   storageRequest `json:"localStorage"`
	S3Storage      storageRequest `json:"s3Storage"`
}

// storageRequest defines storage settings for a backup schedule
type storageRequest struct {
	Enabled     bool   `json:"enabled"`
	Duration    string `json:"duration"`
	KeepForever bool   `json:"keepForever"`
}

// scheduleResponse is the response structure for schedule information
type scheduleResponse struct {
	ID             string          `json:"id"`
	Name           string          `json:"name"`
	BackupType     string          `json:"backupType"`
	CronExpression string          `json:"cronExpression"`
	Enabled        bool            `json:"enabled"`
	LocalStorage   storageResponse `json:"localStorage"`
	S3Storage      storageResponse `json:"s3Storage"`
	CreatedAt      time.Time       `json:"createdAt"`
	UpdatedAt      time.Time       `json:"updatedAt"`
}

// storageResponse defines storage response information
type storageResponse struct {
	Enabled     bool   `json:"enabled"`
	Duration    string `json:"duration"`
	KeepForever bool   `json:"keepForever"`
}

// convertScheduleToResponse converts a BackupSchedule to a scheduleResponse
func convertScheduleToResponse(schedule *dbmeta.BackupSchedule) scheduleResponse {
	resp := scheduleResponse{
		ID:             schedule.ID,
		Name:           schedule.Name,
		BackupType:     schedule.BackupType,
		CronExpression: schedule.CronExpression,
		Enabled:        schedule.Enabled,
		CreatedAt:      schedule.CreatedAt,
		UpdatedAt:      schedule.UpdatedAt,
		LocalStorage: storageResponse{
			Enabled:     false,
			Duration:    "24h",
			KeepForever: false,
		},
		S3Storage: storageResponse{
			Enabled:     false,
			Duration:    "24h",
			KeepForever: false,
		},
	}

	// Process retention policies
	for _, policy := range schedule.RetentionPolicies {
		if policy.StorageType == "local" {
			resp.LocalStorage.Enabled = true
			resp.LocalStorage.Duration = policy.Duration
			resp.LocalStorage.KeepForever = policy.KeepForever
		} else if policy.StorageType == "s3" {
			resp.S3Storage.Enabled = true
			resp.S3Storage.Duration = policy.Duration
			resp.S3Storage.KeepForever = policy.KeepForever
		}
	}

	return resp
}

// createOrUpdateSchedule handles creating or updating a schedule
func (h *ScheduleHandler) createOrUpdateSchedule(w http.ResponseWriter, r *http.Request) {
	var req scheduleRequest
	err := json.NewDecoder(r.Body).Decode(&req)
	if err != nil {
		http.Error(w, "Invalid request: "+err.Error(), http.StatusBadRequest)
		return
	}

	// Validate required fields
	if req.Name == "" || req.BackupType == "" || req.CronExpression == "" {
		http.Error(w, "Missing required fields", http.StatusBadRequest)
		return
	}

	// Create the schedule
	schedule := dbmeta.BackupSchedule{
		Name:           req.Name,
		BackupType:     req.BackupType,
		CronExpression: req.CronExpression,
		Enabled:        req.Enabled,
	}

	// Handle update vs create
	isUpdate := false
	if req.ID != "" {
		// This is an update
		schedule.ID = req.ID
		existing, err := h.scheduleRepo.GetScheduleByID(req.ID)
		if err != nil {
			http.Error(w, "Schedule not found: "+err.Error(), http.StatusNotFound)
			return
		}
		schedule.CreatedAt = existing.CreatedAt
		isUpdate = true
	} else {
		// This is a new schedule, generate ID
		schedule.ID = uuid.New().String()
		schedule.CreatedAt = time.Now()
	}

	schedule.UpdatedAt = time.Now()

	// Add retention policies
	if req.LocalStorage.Enabled {
		schedule.RetentionPolicies = append(schedule.RetentionPolicies, dbmeta.ScheduleRetentionPolicy{
			ScheduleID:  schedule.ID,
			StorageType: "local",
			Duration:    req.LocalStorage.Duration,
			KeepForever: req.LocalStorage.KeepForever,
			CreatedAt:   time.Now(),
		})
	}

	if req.S3Storage.Enabled {
		schedule.RetentionPolicies = append(schedule.RetentionPolicies, dbmeta.ScheduleRetentionPolicy{
			ScheduleID:  schedule.ID,
			StorageType: "s3",
			Duration:    req.S3Storage.Duration,
			KeepForever: req.S3Storage.KeepForever,
			CreatedAt:   time.Now(),
		})
	}

	// Save to database
	if isUpdate {
		err = h.scheduleRepo.UpdateSchedule(&schedule)
	} else {
		err = h.scheduleRepo.CreateSchedule(&schedule)
	}

	if err != nil {
		http.Error(w, "Failed to save schedule: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Update global configuration by reloading database schedules
	go h.reloadSchedulesFromDatabase()

	// Return the schedule
	response := convertScheduleToResponse(&schedule)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// handleDeleteSchedule handles deleting a schedule
func (h *ScheduleHandler) handleDeleteSchedule(w http.ResponseWriter, r *http.Request) {
	// Only allow POST method
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Get schedule ID from query parameters
	scheduleID := r.URL.Query().Get("id")
	if scheduleID == "" {
		http.Error(w, "Schedule ID is required", http.StatusBadRequest)
		return
	}

	if h.scheduleRepo == nil {
		http.Error(w, "Schedule management is not available: database not initialized", http.StatusServiceUnavailable)
		return
	}

	// Check if schedule exists
	exists, err := h.scheduleRepo.ScheduleExists(scheduleID)
	if err != nil {
		http.Error(w, "Failed to check schedule existence: "+err.Error(), http.StatusInternalServerError)
		return
	}
	if !exists {
		http.Error(w, "Schedule not found", http.StatusNotFound)
		return
	}

	// Delete the schedule
	err = h.scheduleRepo.DeleteSchedule(scheduleID)
	if err != nil {
		http.Error(w, "Failed to delete schedule: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Update global configuration by reloading database schedules
	go h.reloadSchedulesFromDatabase()

	// Return success
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"status":  "success",
		"message": "Schedule deleted successfully",
	})
}

// reloadSchedulesFromDatabase reloads schedule configurations from the database
func (h *ScheduleHandler) reloadSchedulesFromDatabase() {
	if metadata.DB == nil {
		log.Println("Cannot reload configuration: database not initialized")
		return
	}

	// Use the handler's schedule repository
	scheduleRepo := h.scheduleRepo
	if scheduleRepo == nil {
		scheduleRepo = dbmeta.NewScheduleRepository(metadata.DB)
	}

	// Get all schedules from the database
	schedules, err := scheduleRepo.GetAllSchedules()
	if err != nil {
		log.Printf("Failed to load schedule configurations from database: %v", err)
		return
	}

	// Convert database schedules to config format
	backupTypes := make(map[string]config.BackupTypeConfig)

	for _, schedule := range schedules {
		if !schedule.Enabled {
			continue // Skip disabled schedules
		}

		// Create backup type config
		backupType := config.BackupTypeConfig{
			Schedule: schedule.CronExpression,
			Local: config.LocalBackupConfig{
				Enabled: false,
				Retention: config.RetentionRule{
					Duration: "24h",
					Forever:  false,
				},
			},
			S3: config.S3BackupConfig{
				Enabled: false,
				Retention: config.RetentionRule{
					Duration: "24h",
					Forever:  false,
				},
			},
		}

		// Process retention policies
		for _, policy := range schedule.RetentionPolicies {
			if policy.StorageType == "local" {
				backupType.Local.Enabled = true
				backupType.Local.Retention.Duration = policy.Duration
				backupType.Local.Retention.Forever = policy.KeepForever
			} else if policy.StorageType == "s3" {
				backupType.S3.Enabled = true
				backupType.S3.Retention.Duration = policy.Duration
				backupType.S3.Retention.Forever = policy.KeepForever
			}
		}

		// Add to map
		backupTypes[schedule.BackupType] = backupType
	}

	// Critical section: update the global configuration
	config.CFG.BackupTypes = backupTypes

	// Reload the scheduler with new configuration
	if h.scheduler != nil {
		if err := h.scheduler.ReloadSchedules(); err != nil {
			log.Printf("Failed to reload scheduler: %v", err)
		}
	}

	log.Printf("Successfully loaded %d schedule configurations from the database", len(schedules))
}
