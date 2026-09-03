package schedule

import (
	"testing"
	"time"

	"github.com/chenbb0128/tuoguan-system-server/internal/modules/masterdata"
	"github.com/chenbb0128/tuoguan-system-server/internal/modules/pickup"
)

func TestGeneratorCreatesDraftAndIsIdempotent(t *testing.T) {
	ctx := t.Context()
	master := masterdata.NewMemoryStore()
	school, err := master.CreateSchool(ctx, masterdata.DefaultOrganizationID, masterdata.CreateSchoolParams{Name: "向阳小学"})
	if err != nil {
		t.Fatal(err)
	}
	term, err := master.CreateAcademicTerm(ctx, masterdata.DefaultOrganizationID, masterdata.CreateAcademicTermParams{Name: "2026秋季", StartsOn: date(2026, 9, 1), EndsOn: date(2027, 1, 31)})
	if err != nil {
		t.Fatal(err)
	}
	class, err := master.CreateSchoolClass(ctx, masterdata.DefaultOrganizationID, masterdata.CreateSchoolClassParams{SchoolID: school.ID, TermID: term.ID, Grade: "一年级", Name: "2班"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := master.CreateStudent(ctx, masterdata.DefaultOrganizationID, masterdata.CreateStudentParams{SchoolID: school.ID, TermID: term.ID, SchoolClassID: class.ID, Name: "小芽", Gender: "unknown"}); err != nil {
		t.Fatal(err)
	}
	schedules := NewMemoryStore()
	item, err := schedules.Create(ctx, masterdata.DefaultOrganizationID, CreateParams{SchoolID: school.ID, SchoolClassID: class.ID, Weekday: time.Monday, PickupMode: PickupModeSchool, TeacherName: "王老师", ExpectedPickupTime: "16:30", EffectiveFrom: date(2026, 9, 1), Enabled: true})
	if err != nil {
		t.Fatal(err)
	}
	pickupStore := pickup.NewMemoryStore()
	generator := NewGenerator(schedules, master, pickupStore)
	result, err := generator.GenerateForDate(ctx, masterdata.DefaultOrganizationID, date(2026, 9, 7))
	if err != nil {
		t.Fatal(err)
	}
	if len(result.CreatedOperationIDs) != 1 || result.CreatedOperationIDs[0] == 0 {
		t.Fatalf("created operations = %+v", result.CreatedOperationIDs)
	}
	operations, err := pickupStore.ListOperations(ctx, masterdata.DefaultOrganizationID)
	if err != nil || len(operations) != 1 {
		t.Fatalf("operations = %+v, err = %v", operations, err)
	}
	if operations[0].ExpectedPickupTime != "16:30" || operations[0].Status != pickup.OperationStatusDraft {
		t.Fatalf("operation = %+v", operations[0])
	}
	second, err := generator.GenerateForDate(ctx, masterdata.DefaultOrganizationID, date(2026, 9, 7))
	if err != nil {
		t.Fatal(err)
	}
	if len(second.CreatedOperationIDs) != 0 || len(second.SkippedScheduleIDs) != 1 || second.SkippedScheduleIDs[0] != item.ID {
		t.Fatalf("second result = %+v", second)
	}
}

func date(year int, month time.Month, day int) time.Time {
	return time.Date(year, month, day, 0, 0, 0, 0, time.UTC)
}
