package metadata

import (
	"errors"
	"fmt"
	"github.com/google/uuid"
	"gorm.io/gorm"
	"time"
)

// ScheduleRepository handles database operations for backup schedules
type ScheduleRepository struct {
	db *gorm.DB
}

// NewScheduleRepository creates a new ScheduleRepository instance
func NewScheduleRepository(db *gorm.DB) *ScheduleRepository {
	return &ScheduleRepository{db: db}
}

// GetAllSchedules retrieves all backup schedules
func (r *ScheduleRepository) GetAllSchedules() ([]BackupSchedule, error) {
	var schedules []BackupSchedule

	err := r.db.Preload("RetentionPolicies").Find(&schedules).Error
	if err != nil {
		return nil, fmt.Errorf("failed to get schedules: %w", err)
	}

	return schedules, nil
}

// GetScheduleByID retrieves a backup schedule by ID
func (r *ScheduleRepository) GetScheduleByID(id string) (*BackupSchedule, error) {
	var schedule BackupSchedule

	err := r.db.Preload("RetentionPolicies").Where("id = ?", id).First(&schedule).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("schedule not found: %s", id)
		}
		return nil, fmt.Errorf("failed to get schedule: %w", err)
	}

	return &schedule, nil
}

// GetScheduleByName retrieves a backup schedule by name
func (r *ScheduleRepository) GetScheduleByName(name string) (*BackupSchedule, error) {
	var schedule BackupSchedule

	err := r.db.Preload("RetentionPolicies").Where("name = ?", name).First(&schedule).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("schedule not found: %s", name)
		}
		return nil, fmt.Errorf("failed to get schedule: %w", err)
	}

	return &schedule, nil
}

// GetEnabledSchedules retrieves all enabled backup schedules
func (r *ScheduleRepository) GetEnabledSchedules() ([]BackupSchedule, error) {
	var schedules []BackupSchedule

	err := r.db.Preload("RetentionPolicies").Where("enabled = ?", true).Find(&schedules).Error
	if err != nil {
		return nil, fmt.Errorf("failed to get enabled schedules: %w", err)
	}

	return schedules, nil
}

// CreateSchedule creates a new backup schedule
func (r *ScheduleRepository) CreateSchedule(schedule *BackupSchedule) error {
	// Generate a new UUID if not provided
	if schedule.ID == "" {
		schedule.ID = uuid.New().String()
	}

	// Set timestamps
	now := time.Now()
	schedule.CreatedAt = now
	schedule.UpdatedAt = now

	// Start a transaction
	tx := r.db.Begin()
	if tx.Error != nil {
		return fmt.Errorf("failed to begin transaction: %w", tx.Error)
	}

	// Create the schedule
	if err := tx.Create(schedule).Error; err != nil {
		tx.Rollback()
		return fmt.Errorf("failed to create schedule: %w", err)
	}

	// Commit the transaction
	if err := tx.Commit().Error; err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}

// UpdateSchedule updates an existing backup schedule
func (r *ScheduleRepository) UpdateSchedule(schedule *BackupSchedule) error {
	// Check if schedule exists
	exists, err := r.ScheduleExists(schedule.ID)
	if err != nil {
		return err
	}
	if !exists {
		return fmt.Errorf("schedule not found: %s", schedule.ID)
	}

	// Update timestamp
	schedule.UpdatedAt = time.Now()

	// Start a transaction
	tx := r.db.Begin()
	if tx.Error != nil {
		return fmt.Errorf("failed to begin transaction: %w", tx.Error)
	}

	// Delete existing retention policies to replace them
	if err := tx.Where("schedule_id = ?", schedule.ID).Delete(&ScheduleRetentionPolicy{}).Error; err != nil {
		tx.Rollback()
		return fmt.Errorf("failed to delete existing retention policies: %w", err)
	}

	// Update the schedule
	if err := tx.Save(schedule).Error; err != nil {
		tx.Rollback()
		return fmt.Errorf("failed to update schedule: %w", err)
	}

	// Commit the transaction
	if err := tx.Commit().Error; err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}

// DeleteSchedule deletes a backup schedule
func (r *ScheduleRepository) DeleteSchedule(id string) error {
	// Start a transaction
	tx := r.db.Begin()
	if tx.Error != nil {
		return fmt.Errorf("failed to begin transaction: %w", tx.Error)
	}

	// Delete the schedule (cascade will delete related retention policies)
	if err := tx.Delete(&BackupSchedule{ID: id}).Error; err != nil {
		tx.Rollback()
		return fmt.Errorf("failed to delete schedule: %w", err)
	}

	// Commit the transaction
	if err := tx.Commit().Error; err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}

// ScheduleExists checks if a schedule with the given ID exists
func (r *ScheduleRepository) ScheduleExists(id string) (bool, error) {
	var count int64
	err := r.db.Model(&BackupSchedule{}).Where("id = ?", id).Count(&count).Error
	if err != nil {
		return false, fmt.Errorf("failed to check if schedule exists: %w", err)
	}
	return count > 0, nil
}

// ScheduleExistsByName checks if a schedule with the given name exists
func (r *ScheduleRepository) ScheduleExistsByName(name string) (bool, error) {
	var count int64
	err := r.db.Model(&BackupSchedule{}).Where("name = ?", name).Count(&count).Error
	if err != nil {
		return false, fmt.Errorf("failed to check if schedule exists: %w", err)
	}
	return count > 0, nil
}
