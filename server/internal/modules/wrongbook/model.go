package wrongbook

import "time"

const (
	QuestionStatusActive   = "active"
	QuestionStatusMastered = "mastered"
	QuestionStatusArchived = "archived"

	PaperStatusGenerated = "generated"
	PaperStatusAssigned  = "assigned"
	PaperStatusArchived  = "archived"

	PaperSourceTeacher = "teacher"
	PaperSourceParent  = "parent"
	PaperSourceSystem  = "system"

	GeneratedByStaff  = "staff"
	GeneratedByParent = "parent"
	GeneratedBySystem = "system"
)

type Question struct {
	ID                   uint64
	OrganizationID       uint64
	StudentID            uint64
	StudentName          string
	Subject              string
	QuestionText         string
	AnswerText           string
	Explanation          string
	KnowledgePoint       string
	SourceImageURL       string
	SourceHomeworkTaskID *uint64
	TeacherNote          string
	Status               string
	CreatedByUserID      *uint64
	CreatedByName        string
	LastReviewedAt       *time.Time
	CreatedAt            time.Time
	UpdatedAt            time.Time
}

type Paper struct {
	ID                uint64
	OrganizationID    uint64
	StudentID         uint64
	StudentName       string
	Title             string
	Source            string
	Status            string
	GeneratedByType   string
	GeneratedByUserID *uint64
	QuestionCount     int
	Questions         []Question
	CreatedAt         time.Time
	UpdatedAt         time.Time
}

type ExtractedQuestion struct {
	TempID         string  `json:"temp_id"`
	Subject        string  `json:"subject"`
	QuestionText   string  `json:"question_text"`
	AnswerText     string  `json:"answer_text"`
	Explanation    string  `json:"explanation"`
	KnowledgePoint string  `json:"knowledge_point"`
	Confidence     float64 `json:"confidence"`
}

type ListQuestionsParams struct {
	StudentID uint64
	Subject   string
	Status    string
	Keyword   string
}

type CreateQuestionParams struct {
	StudentID            uint64
	Subject              string
	QuestionText         string
	AnswerText           string
	Explanation          string
	KnowledgePoint       string
	SourceImageURL       string
	SourceHomeworkTaskID *uint64
	TeacherNote          string
	CreatedByUserID      *uint64
	CreatedByName        string
}

type BulkCreateQuestionsParams struct {
	Items []CreateQuestionParams
}

type UpdateQuestionParams struct {
	ID             uint64
	Subject        string
	QuestionText   string
	AnswerText     string
	Explanation    string
	KnowledgePoint string
	TeacherNote    string
	Status         string
}

type ListPapersParams struct {
	StudentID uint64
}

type CreatePaperParams struct {
	StudentID         uint64
	Title             string
	QuestionIDs       []uint64
	Source            string
	GeneratedByType   string
	GeneratedByUserID *uint64
}
