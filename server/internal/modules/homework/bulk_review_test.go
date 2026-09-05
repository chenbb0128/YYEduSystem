package homework

import (
	"context"
	"testing"
	"time"
)

func TestMemoryBulkReviewIsAtomicAndUpdatesAllStudents(t *testing.T) {
	store := NewMemoryStore()
	task, err := store.CreateTask(context.Background(), 1, CreateTaskParams{HomeworkDate: time.Date(2026, 9, 2, 0, 0, 0, 0, time.UTC), SchoolID: 1, SchoolClassID: 1, Subject: "数学", Content: "练习"}, []StudentRef{{ID: 11, Name: "小明"}, {ID: 12, Name: "小红"}})
	if err != nil {
		t.Fatal(err)
	}
	_, err = store.BulkReviewStudents(context.Background(), 1, BulkReviewStudentsParams{TaskID: task.ID, Items: []BulkReviewItem{{StudentID: 11, Status: StudentStatusCompleted}, {StudentID: 999, Status: StudentStatusIncomplete}}})
	if err == nil {
		t.Fatal("invalid batch should fail")
	}
	items, err := store.ListTaskStudents(context.Background(), 1, task.ID)
	if err != nil {
		t.Fatal(err)
	}
	for _, item := range items {
		if item.Status != StudentStatusPending {
			t.Fatalf("invalid batch changed student %d to %s", item.StudentID, item.Status)
		}
	}

	updated, err := store.BulkReviewStudents(context.Background(), 1, BulkReviewStudentsParams{TaskID: task.ID, Items: []BulkReviewItem{{StudentID: 11, Status: StudentStatusCompleted, CorrectionNote: "完成"}, {StudentID: 12, Status: StudentStatusIncomplete, CorrectionNote: "订正"}}})
	if err != nil {
		t.Fatal(err)
	}
	if len(updated) != 2 || updated[0].Status != StudentStatusCompleted || updated[1].Status != StudentStatusIncomplete {
		t.Fatalf("updated batch = %+v", updated)
	}
}
