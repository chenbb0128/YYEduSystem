package schedule

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/chenbb0128/tuoguan-system-server/internal/modules/masterdata"
	"github.com/chenbb0128/tuoguan-system-server/internal/modules/pickup"
)

// Generator turns enabled weekly schedules into draft pickup operations. It
// intentionally does not confirm or start a task: a teacher must still review
// the roster before departure. Confirmation is the point at which parents are
// notified, so an automatically generated draft can never send a wrong notice.
type Generator struct {
	schedules  Store
	masterData masterdata.Store
	pickup     pickup.Store
}

func NewGenerator(schedules Store, masterData masterdata.Store, pickupStore pickup.Store) *Generator {
	return &Generator{schedules: schedules, masterData: masterData, pickup: pickupStore}
}

func (g *Generator) GenerateForDate(ctx context.Context, orgID uint64, date time.Time) (GenerationResult, error) {
	items, err := g.schedules.List(ctx, orgID)
	if err != nil {
		return GenerationResult{}, err
	}
	return g.Generate(ctx, orgID, date, items)
}

func (g *Generator) Generate(ctx context.Context, orgID uint64, date time.Time, schedules []PickupSchedule) (GenerationResult, error) {
	date = date.UTC().Truncate(24 * time.Hour)
	result := GenerationResult{Date: date, CreatedOperationIDs: []uint64{}, SkippedScheduleIDs: []uint64{}, SkippedReasons: map[uint64]string{}}
	if g.masterData == nil || g.pickup == nil {
		return result, fmt.Errorf("schedule: generator dependencies are required")
	}
	classes, err := g.masterData.ListSchoolClasses(ctx, orgID)
	if err != nil {
		return result, err
	}
	classByID := make(map[uint64]masterdata.SchoolClass, len(classes))
	for _, item := range classes {
		classByID[item.ID] = item
	}
	students, err := g.masterData.ListStudents(ctx, orgID)
	if err != nil {
		return result, err
	}
	operations, err := g.pickup.ListOperations(ctx, orgID)
	if err != nil {
		return result, err
	}
	existing := make(map[string]struct{})
	for _, item := range operations {
		if sameDay(item.OperationDate, date) && item.Status != pickup.OperationStatusCancelled {
			existing[operationKey(item.SchoolClassID)] = struct{}{}
		}
	}
	for _, item := range schedules {
		if !item.Enabled || item.Weekday != date.Weekday() || date.Before(item.EffectiveFrom.UTC().Truncate(24*time.Hour)) || (item.EffectiveTo != nil && date.After(item.EffectiveTo.UTC().Truncate(24*time.Hour))) {
			continue
		}
		if _, exists := existing[operationKey(item.SchoolClassID)]; exists {
			result.SkippedScheduleIDs = append(result.SkippedScheduleIDs, item.ID)
			result.SkippedReasons[item.ID] = "当天接送任务已存在"
			continue
		}
		class, exists := classByID[item.SchoolClassID]
		if !exists || class.Status != "active" {
			result.SkippedScheduleIDs = append(result.SkippedScheduleIDs, item.ID)
			result.SkippedReasons[item.ID] = "学校班级不存在或已停用"
			continue
		}
		roster := make([]pickup.StudentRef, 0)
		for _, student := range students {
			if student.Status != "active" || student.SchoolClassID != item.SchoolClassID {
				continue
			}
			if item.CareClassID != nil && (student.CareClassID == nil || *student.CareClassID != *item.CareClassID) {
				continue
			}
			roster = append(roster, pickup.StudentRef{ID: student.ID, Name: student.Name})
		}
		if len(roster) == 0 {
			result.SkippedScheduleIDs = append(result.SkippedScheduleIDs, item.ID)
			result.SkippedReasons[item.ID] = "班级暂无在托学生"
			continue
		}
		operation, createErr := g.pickup.CreateOperation(ctx, orgID, pickup.CreateOperationParams{
			OperationDate: date, PickupMode: item.PickupMode, SchoolID: item.SchoolID, SchoolClassID: item.SchoolClassID,
			CareClassID: item.CareClassID, TeacherUserID: item.TeacherUserID, TeacherName: strings.TrimSpace(item.TeacherName),
			ExpectedPickupTime: strings.TrimSpace(item.ExpectedPickupTime), Notes: strings.TrimSpace(item.Notes),
		}, roster)
		if createErr != nil {
			if createErr == pickup.ErrConflict {
				result.SkippedScheduleIDs = append(result.SkippedScheduleIDs, item.ID)
				result.SkippedReasons[item.ID] = "当天接送任务已存在"
				continue
			}
			return result, createErr
		}
		result.CreatedOperationIDs = append(result.CreatedOperationIDs, operation.ID)
		existing[operationKey(item.SchoolClassID)] = struct{}{}
	}
	return result, nil
}

func operationKey(classID uint64) string { return fmt.Sprintf("%d", classID) }
func sameDay(left, right time.Time) bool {
	return left.UTC().Format("2006-01-02") == right.UTC().Format("2006-01-02")
}
