package wrongbook

import (
	"errors"
	"fmt"
	"io"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/chenbb0128/tuoguan-system-server/internal/modules/assignment"
	auditmodule "github.com/chenbb0128/tuoguan-system-server/internal/modules/audit"
	"github.com/chenbb0128/tuoguan-system-server/internal/modules/identity"
	"github.com/chenbb0128/tuoguan-system-server/internal/modules/masterdata"
	"github.com/chenbb0128/tuoguan-system-server/internal/modules/parent"
	ocrplatform "github.com/chenbb0128/tuoguan-system-server/internal/platform/ocr"
	"github.com/chenbb0128/tuoguan-system-server/internal/platform/storage"
	"github.com/chenbb0128/tuoguan-system-server/internal/transport/httpapi/request"
	"github.com/chenbb0128/tuoguan-system-server/internal/transport/httpapi/response"
)

type Handler struct {
	store        Store
	masterData   masterdata.Store
	assignments  assignment.Store
	parents      parent.Store
	users        identity.UserStore
	audit        auditmodule.Writer
	ocrClient    ocrplatform.Client
	uploadReader storage.FileReader
	orgID        uint64
}

func NewHandler(store Store, masterData masterdata.Store) *Handler {
	return &Handler{store: store, masterData: masterData, orgID: masterdata.DefaultOrganizationID}
}

func (h *Handler) SetStaffScope(assignments assignment.Store, users identity.UserStore) {
	h.assignments = assignments
	h.users = users
}

func (h *Handler) SetParentStore(parents parent.Store) { h.parents = parents }

func (h *Handler) SetAuditWriter(writer auditmodule.Writer) { h.audit = writer }

func (h *Handler) SetOCRClient(client ocrplatform.Client) { h.ocrClient = client }

func (h *Handler) SetUploadReader(reader storage.FileReader) { h.uploadReader = reader }

func (h *Handler) RegisterStaffRoutes(api *gin.RouterGroup) {
	api.GET("/wrong-questions", h.listQuestions)
	api.POST("/wrong-questions/extract", h.extractQuestions)
	api.POST("/wrong-questions", h.createQuestion)
	api.POST("/wrong-questions/bulk", h.bulkCreateQuestions)
	api.PUT("/wrong-questions/:id", h.updateQuestion)
	api.GET("/wrong-papers", h.listPapers)
	api.GET("/wrong-papers/:id", h.getPaper)
	api.POST("/wrong-papers", h.createPaper)
}

func (h *Handler) RegisterParentRoutes(api *gin.RouterGroup) {
	api.GET("/parent/students/:student_id/wrong-questions", h.listParentQuestions)
	api.GET("/parent/students/:student_id/wrong-papers", h.listParentPapers)
	api.GET("/parent/students/:student_id/wrong-papers/:id", h.getParentPaper)
	api.POST("/parent/students/:student_id/wrong-papers", h.createParentPaper)
}

type questionView struct {
	ID                   uint64  `json:"id"`
	StudentID            uint64  `json:"student_id"`
	StudentName          string  `json:"student_name"`
	Subject              string  `json:"subject"`
	QuestionText         string  `json:"question_text"`
	AnswerText           string  `json:"answer_text"`
	Explanation          string  `json:"explanation"`
	KnowledgePoint       string  `json:"knowledge_point"`
	SourceImageURL       string  `json:"source_image_url"`
	SourceHomeworkTaskID *uint64 `json:"source_homework_task_id,omitempty"`
	TeacherNote          string  `json:"teacher_note"`
	Status               string  `json:"status"`
	CreatedByUserID      *uint64 `json:"created_by_user_id,omitempty"`
	CreatedByName        string  `json:"created_by_name"`
	LastReviewedAt       *string `json:"last_reviewed_at,omitempty"`
	CreatedAt            string  `json:"created_at"`
	UpdatedAt            string  `json:"updated_at"`
}

type paperView struct {
	ID                uint64         `json:"id"`
	StudentID         uint64         `json:"student_id"`
	StudentName       string         `json:"student_name"`
	Title             string         `json:"title"`
	Source            string         `json:"source"`
	Status            string         `json:"status"`
	GeneratedByType   string         `json:"generated_by_type"`
	GeneratedByUserID *uint64        `json:"generated_by_user_id,omitempty"`
	QuestionCount     int            `json:"question_count"`
	Questions         []questionView `json:"questions,omitempty"`
	CreatedAt         string         `json:"created_at"`
	UpdatedAt         string         `json:"updated_at"`
}

type extractQuestionsRequest struct {
	ImageURL   string `json:"image_url"`
	SourceText string `json:"source_text"`
	Subject    string `json:"subject"`
}

func (r extractQuestionsRequest) Validate() []response.ValidationDetail {
	details := make([]response.ValidationDetail, 0)
	if strings.TrimSpace(r.ImageURL) == "" && strings.TrimSpace(r.SourceText) == "" {
		details = append(details, response.ValidationDetail{Field: "image_url", Reason: "required"})
	}
	if len([]rune(r.SourceText)) > 8000 {
		details = append(details, response.ValidationDetail{Field: "source_text", Reason: "too_long"})
	}
	return details
}

type createQuestionRequest struct {
	StudentID            uint64  `json:"student_id"`
	Subject              string  `json:"subject"`
	QuestionText         string  `json:"question_text"`
	AnswerText           string  `json:"answer_text"`
	Explanation          string  `json:"explanation"`
	KnowledgePoint       string  `json:"knowledge_point"`
	SourceImageURL       string  `json:"source_image_url"`
	SourceHomeworkTaskID *uint64 `json:"source_homework_task_id"`
	TeacherNote          string  `json:"teacher_note"`
}

func (r createQuestionRequest) Validate() []response.ValidationDetail {
	return validateQuestionFields(r.StudentID, r.QuestionText, r.Subject, r.AnswerText, r.Explanation, r.KnowledgePoint, r.TeacherNote, r.SourceImageURL)
}

type bulkCreateQuestionsRequest struct {
	Items []createQuestionRequest `json:"items"`
}

func (r bulkCreateQuestionsRequest) Validate() []response.ValidationDetail {
	if len(r.Items) == 0 {
		return []response.ValidationDetail{{Field: "items", Reason: "required"}}
	}
	if len(r.Items) > 100 {
		return []response.ValidationDetail{{Field: "items", Reason: "too_many"}}
	}
	details := make([]response.ValidationDetail, 0)
	for _, item := range r.Items {
		details = append(details, validateQuestionFields(item.StudentID, item.QuestionText, item.Subject, item.AnswerText, item.Explanation, item.KnowledgePoint, item.TeacherNote, item.SourceImageURL)...)
	}
	return details
}

type updateQuestionRequest struct {
	Subject        string `json:"subject"`
	QuestionText   string `json:"question_text"`
	AnswerText     string `json:"answer_text"`
	Explanation    string `json:"explanation"`
	KnowledgePoint string `json:"knowledge_point"`
	TeacherNote    string `json:"teacher_note"`
	Status         string `json:"status"`
}

func (r updateQuestionRequest) Validate() []response.ValidationDetail {
	details := validateQuestionFields(1, r.QuestionText, r.Subject, r.AnswerText, r.Explanation, r.KnowledgePoint, r.TeacherNote, "")
	if !validQuestionStatus(r.Status) {
		details = append(details, response.ValidationDetail{Field: "status", Reason: "invalid_value"})
	}
	return details
}

type createPaperRequest struct {
	StudentID   uint64   `json:"student_id"`
	Title       string   `json:"title"`
	QuestionIDs []uint64 `json:"question_ids"`
}

func (r createPaperRequest) Validate() []response.ValidationDetail {
	return validatePaperFields(r.StudentID, r.Title, r.QuestionIDs)
}

type createParentPaperRequest struct {
	Title       string   `json:"title"`
	QuestionIDs []uint64 `json:"question_ids"`
}

func (r createParentPaperRequest) Validate() []response.ValidationDetail {
	return validatePaperFields(1, r.Title, r.QuestionIDs)
}

func (h *Handler) listQuestions(c *gin.Context) {
	params, ok := h.questionQuery(c)
	if !ok {
		return
	}
	items, err := h.store.ListQuestions(c.Request.Context(), orgID(c, h.orgID), params)
	if err != nil {
		respondStoreError(c, err)
		return
	}
	items, err = h.filterQuestionsForPrincipal(c, items)
	if err != nil {
		respondAccessError(c, err)
		return
	}
	response.OK(c, gin.H{"items": toQuestionViews(items), "total": len(items)})
}

func (h *Handler) listParentQuestions(c *gin.Context) {
	studentID, ok := parsePathID(c, "student_id")
	if !ok {
		return
	}
	if !h.ensureParentChild(c, studentID) {
		return
	}
	params := ListQuestionsParams{StudentID: studentID, Status: strings.TrimSpace(c.Query("status")), Subject: strings.TrimSpace(c.Query("subject")), Keyword: strings.TrimSpace(c.Query("keyword"))}
	items, err := h.store.ListQuestions(c.Request.Context(), orgID(c, h.orgID), params)
	if err != nil {
		respondStoreError(c, err)
		return
	}
	response.OK(c, gin.H{"items": toQuestionViews(items), "total": len(items)})
}

func (h *Handler) extractQuestions(c *gin.Context) {
	if !canWriteWrongbook(c) {
		return
	}
	var req extractQuestionsRequest
	if err := request.BindJSON(c, &req); err != nil {
		response.Error(c, err)
		return
	}
	sourceText := strings.TrimSpace(req.SourceText)
	ocrUsed := false
	ocrProvider := ""
	ocrAction := ""
	ocrConfidence := 0.0
	warning := ""
	if sourceText == "" && strings.TrimSpace(req.ImageURL) != "" {
		result, err := h.extractTextFromImage(c, strings.TrimSpace(req.ImageURL))
		if err != nil {
			warning = "OCR识别暂不可用，请老师手动校对题目内容"
		} else {
			sourceText = result.Text
			ocrUsed = true
			ocrProvider = result.Provider
			ocrAction = result.Action
			ocrConfidence = result.Confidence
		}
	}
	candidates := extractQuestionCandidates(sourceText, req.Subject)
	if ocrUsed && ocrConfidence > 0 {
		for index := range candidates {
			candidates[index].Confidence = ocrConfidence
		}
	}
	mocked := false
	if len(candidates) == 0 {
		mocked = true
		candidates = fallbackQuestionCandidates(req.Subject)
	}
	payload := gin.H{
		"items":          candidates,
		"total":          len(candidates),
		"image_url":      strings.TrimSpace(req.ImageURL),
		"mocked":         mocked,
		"ocr_enabled":    h.ocrClient != nil,
		"ocr_used":       ocrUsed,
		"ocr_provider":   ocrProvider,
		"ocr_action":     ocrAction,
		"ocr_confidence": ocrConfidence,
	}
	if warning != "" {
		payload["warning"] = warning
	}
	response.OK(c, payload)
}

func (h *Handler) extractTextFromImage(c *gin.Context, imageURL string) (ocrplatform.TextResult, error) {
	if h.ocrClient == nil {
		return ocrplatform.TextResult{}, errors.New("wrongbook: OCR client is not configured")
	}
	if h.uploadReader == nil {
		return ocrplatform.TextResult{}, errors.New("wrongbook: upload reader is not configured")
	}
	reader, contentType, err := h.uploadReader.OpenURL(imageURL)
	if err != nil {
		return ocrplatform.TextResult{}, err
	}
	if closer, ok := reader.(io.Closer); ok {
		defer closer.Close()
	}
	return h.ocrClient.ExtractText(c.Request.Context(), reader, contentType)
}

func (h *Handler) createQuestion(c *gin.Context) {
	if !canWriteWrongbook(c) {
		return
	}
	var req createQuestionRequest
	if err := request.BindJSON(c, &req); err != nil {
		response.Error(c, err)
		return
	}
	student, ok := h.ensureStaffStudent(c, req.StudentID)
	if !ok {
		return
	}
	createdBy, createdByName, creatorErr := h.currentCreator(c)
	if creatorErr != nil {
		respondAccessError(c, creatorErr)
		return
	}
	item, err := h.store.CreateQuestion(c.Request.Context(), orgID(c, h.orgID), req.toCreateParams(student, createdBy, createdByName))
	if err != nil {
		respondStoreError(c, err)
		return
	}
	h.recordAudit(c, "wrongbook.question.create", "wrong_question", item.ID)
	response.Created(c, "/api/v1/wrong-questions/"+strconv.FormatUint(item.ID, 10), toQuestionView(item))
}

func (h *Handler) bulkCreateQuestions(c *gin.Context) {
	if !canWriteWrongbook(c) {
		return
	}
	var req bulkCreateQuestionsRequest
	if err := request.BindJSON(c, &req); err != nil {
		response.Error(c, err)
		return
	}
	createdBy, createdByName, creatorErr := h.currentCreator(c)
	if creatorErr != nil {
		respondAccessError(c, creatorErr)
		return
	}
	items := make([]CreateQuestionParams, 0, len(req.Items))
	for _, item := range req.Items {
		student, ok := h.ensureStaffStudent(c, item.StudentID)
		if !ok {
			return
		}
		items = append(items, item.toCreateParams(student, createdBy, createdByName))
	}
	created, err := h.store.BulkCreateQuestions(c.Request.Context(), orgID(c, h.orgID), BulkCreateQuestionsParams{Items: items})
	if err != nil {
		respondStoreError(c, err)
		return
	}
	h.recordAudit(c, "wrongbook.question.bulk_create", "wrong_question", 0)
	response.OK(c, gin.H{"items": toQuestionViews(created), "total": len(created)})
}

func (h *Handler) updateQuestion(c *gin.Context) {
	if !canWriteWrongbook(c) {
		return
	}
	id, ok := parsePathID(c, "id")
	if !ok {
		return
	}
	current, err := h.store.FindQuestion(c.Request.Context(), orgID(c, h.orgID), id)
	if err != nil {
		respondStoreError(c, err)
		return
	}
	if _, ok := h.ensureStaffStudent(c, current.StudentID); !ok {
		return
	}
	var req updateQuestionRequest
	if err := request.BindJSON(c, &req); err != nil {
		response.Error(c, err)
		return
	}
	updated, err := h.store.UpdateQuestion(c.Request.Context(), orgID(c, h.orgID), UpdateQuestionParams{ID: id, Subject: normalizeSubject(req.Subject), QuestionText: strings.TrimSpace(req.QuestionText), AnswerText: strings.TrimSpace(req.AnswerText), Explanation: strings.TrimSpace(req.Explanation), KnowledgePoint: strings.TrimSpace(req.KnowledgePoint), TeacherNote: strings.TrimSpace(req.TeacherNote), Status: req.Status})
	if err != nil {
		respondStoreError(c, err)
		return
	}
	h.recordAudit(c, "wrongbook.question.update", "wrong_question", updated.ID)
	response.OK(c, toQuestionView(updated))
}

func (h *Handler) listPapers(c *gin.Context) {
	studentID, ok := parseOptionalUint(c, "student_id")
	if !ok {
		return
	}
	items, err := h.store.ListPapers(c.Request.Context(), orgID(c, h.orgID), ListPapersParams{StudentID: studentID})
	if err != nil {
		respondStoreError(c, err)
		return
	}
	items, err = h.filterPapersForPrincipal(c, items)
	if err != nil {
		respondAccessError(c, err)
		return
	}
	response.OK(c, gin.H{"items": toPaperViews(items), "total": len(items)})
}

func (h *Handler) listParentPapers(c *gin.Context) {
	studentID, ok := parsePathID(c, "student_id")
	if !ok {
		return
	}
	if !h.ensureParentChild(c, studentID) {
		return
	}
	items, err := h.store.ListPapers(c.Request.Context(), orgID(c, h.orgID), ListPapersParams{StudentID: studentID})
	if err != nil {
		respondStoreError(c, err)
		return
	}
	response.OK(c, gin.H{"items": toPaperViews(items), "total": len(items)})
}

func (h *Handler) getPaper(c *gin.Context) {
	id, ok := parsePathID(c, "id")
	if !ok {
		return
	}
	paper, err := h.store.FindPaper(c.Request.Context(), orgID(c, h.orgID), id)
	if err != nil {
		respondStoreError(c, err)
		return
	}
	if _, ok := h.ensureStaffStudent(c, paper.StudentID); !ok {
		return
	}
	response.OK(c, toPaperView(paper, true))
}

func (h *Handler) getParentPaper(c *gin.Context) {
	studentID, ok := parsePathID(c, "student_id")
	if !ok {
		return
	}
	if !h.ensureParentChild(c, studentID) {
		return
	}
	id, ok := parsePathID(c, "id")
	if !ok {
		return
	}
	paper, err := h.store.FindPaper(c.Request.Context(), orgID(c, h.orgID), id)
	if err != nil {
		respondStoreError(c, err)
		return
	}
	if paper.StudentID != studentID {
		response.Error(c, response.NotFound())
		return
	}
	response.OK(c, toPaperView(paper, true))
}

func (h *Handler) createPaper(c *gin.Context) {
	var req createPaperRequest
	if err := request.BindJSON(c, &req); err != nil {
		response.Error(c, err)
		return
	}
	if !canWriteWrongbook(c) {
		return
	}
	if _, ok := h.ensureStaffStudent(c, req.StudentID); !ok {
		return
	}
	principal, scoped := identity.PrincipalFromContext(c.Request.Context())
	var generatedBy *uint64
	if scoped {
		generatedBy = &principal.SubjectID
	}
	paper, err := h.store.CreatePaper(c.Request.Context(), orgID(c, h.orgID), CreatePaperParams{StudentID: req.StudentID, Title: strings.TrimSpace(req.Title), QuestionIDs: req.QuestionIDs, Source: PaperSourceTeacher, GeneratedByType: GeneratedByStaff, GeneratedByUserID: generatedBy})
	if err != nil {
		respondStoreError(c, err)
		return
	}
	h.recordAudit(c, "wrongbook.paper.create", "wrong_paper", paper.ID)
	response.Created(c, "/api/v1/wrong-papers/"+strconv.FormatUint(paper.ID, 10), toPaperView(paper, true))
}

func (h *Handler) createParentPaper(c *gin.Context) {
	studentID, ok := parsePathID(c, "student_id")
	if !ok {
		return
	}
	if !h.ensureParentChild(c, studentID) {
		return
	}
	var req createParentPaperRequest
	if err := request.BindJSON(c, &req); err != nil {
		response.Error(c, err)
		return
	}
	principal, _ := identity.PrincipalFromContext(c.Request.Context())
	paper, err := h.store.CreatePaper(c.Request.Context(), orgID(c, h.orgID), CreatePaperParams{StudentID: studentID, Title: strings.TrimSpace(req.Title), QuestionIDs: req.QuestionIDs, Source: PaperSourceParent, GeneratedByType: GeneratedByParent, GeneratedByUserID: &principal.SubjectID})
	if err != nil {
		respondStoreError(c, err)
		return
	}
	response.Created(c, "/api/v1/parent/students/"+strconv.FormatUint(studentID, 10)+"/wrong-papers/"+strconv.FormatUint(paper.ID, 10), toPaperView(paper, true))
}

func (r createQuestionRequest) toCreateParams(student masterdata.Student, createdBy *uint64, createdByName string) CreateQuestionParams {
	return CreateQuestionParams{StudentID: student.ID, Subject: normalizeSubject(r.Subject), QuestionText: strings.TrimSpace(r.QuestionText), AnswerText: strings.TrimSpace(r.AnswerText), Explanation: strings.TrimSpace(r.Explanation), KnowledgePoint: strings.TrimSpace(r.KnowledgePoint), SourceImageURL: strings.TrimSpace(r.SourceImageURL), SourceHomeworkTaskID: r.SourceHomeworkTaskID, TeacherNote: strings.TrimSpace(r.TeacherNote), CreatedByUserID: createdBy, CreatedByName: createdByName}
}

func (h *Handler) questionQuery(c *gin.Context) (ListQuestionsParams, bool) {
	studentID, ok := parseOptionalUint(c, "student_id")
	if !ok {
		return ListQuestionsParams{}, false
	}
	return ListQuestionsParams{StudentID: studentID, Subject: strings.TrimSpace(c.Query("subject")), Status: strings.TrimSpace(c.Query("status")), Keyword: strings.TrimSpace(c.Query("keyword"))}, true
}

func (h *Handler) ensureStaffStudent(c *gin.Context, studentID uint64) (masterdata.Student, bool) {
	student, err := h.masterData.FindStudent(c.Request.Context(), orgID(c, h.orgID), studentID)
	if err != nil {
		if errors.Is(err, masterdata.ErrNotFound) {
			response.Error(c, response.NotFound())
			return masterdata.Student{}, false
		}
		response.Error(c, response.Internal(err))
		return masterdata.Student{}, false
	}
	if err := h.ensureClassAccess(c, student.SchoolClassID); err != nil {
		respondAccessError(c, err)
		return masterdata.Student{}, false
	}
	return student, true
}

func (h *Handler) ensureClassAccess(c *gin.Context, schoolClassID uint64) error {
	principal, ok := identity.PrincipalFromContext(c.Request.Context())
	if !ok || principal.Kind != identity.PrincipalKindUser || principal.Role != identity.UserRoleTeacher || h.assignments == nil {
		return nil
	}
	item, err := h.assignments.FindByPair(c.Request.Context(), orgID(c, h.orgID), principal.SubjectID, schoolClassID)
	if err != nil {
		if errors.Is(err, assignment.ErrNotFound) {
			return ErrNotFound
		}
		return err
	}
	if item.Status != assignment.AssignmentStatusActive {
		return ErrNotFound
	}
	return nil
}

func (h *Handler) ensureParentChild(c *gin.Context, studentID uint64) bool {
	principal, ok := identity.PrincipalFromContext(c.Request.Context())
	if !ok || principal.Kind != identity.PrincipalKindParent {
		response.Error(c, response.Unauthorized())
		return false
	}
	if h.parents == nil {
		response.Error(c, response.Internal(errors.New("家长绑定服务未配置")))
		return false
	}
	bindings, err := h.parents.ListBindings(c.Request.Context(), orgID(c, h.orgID), principal.SubjectID)
	if err != nil {
		response.Error(c, response.Internal(err))
		return false
	}
	for _, item := range bindings {
		if item.StudentID == studentID {
			return true
		}
	}
	response.Error(c, response.NotFound())
	return false
}

func (h *Handler) filterQuestionsForPrincipal(c *gin.Context, items []Question) ([]Question, error) {
	principal, ok := identity.PrincipalFromContext(c.Request.Context())
	if !ok || principal.Kind != identity.PrincipalKindUser || principal.Role != identity.UserRoleTeacher || h.assignments == nil {
		return items, nil
	}
	assigned, err := h.assignments.List(c.Request.Context(), orgID(c, h.orgID), principal.SubjectID, 0)
	if err != nil {
		return nil, err
	}
	classes := make(map[uint64]struct{}, len(assigned))
	for _, item := range assigned {
		if item.Status == assignment.AssignmentStatusActive {
			classes[item.SchoolClassID] = struct{}{}
		}
	}
	filtered := make([]Question, 0, len(items))
	for _, item := range items {
		student, err := h.masterData.FindStudent(c.Request.Context(), orgID(c, h.orgID), item.StudentID)
		if err != nil {
			return nil, err
		}
		if _, ok := classes[student.SchoolClassID]; ok {
			filtered = append(filtered, item)
		}
	}
	return filtered, nil
}

func (h *Handler) filterPapersForPrincipal(c *gin.Context, items []Paper) ([]Paper, error) {
	principal, ok := identity.PrincipalFromContext(c.Request.Context())
	if !ok || principal.Kind != identity.PrincipalKindUser || principal.Role != identity.UserRoleTeacher || h.assignments == nil {
		return items, nil
	}
	assigned, err := h.assignments.List(c.Request.Context(), orgID(c, h.orgID), principal.SubjectID, 0)
	if err != nil {
		return nil, err
	}
	classes := make(map[uint64]struct{}, len(assigned))
	for _, item := range assigned {
		if item.Status == assignment.AssignmentStatusActive {
			classes[item.SchoolClassID] = struct{}{}
		}
	}
	filtered := make([]Paper, 0, len(items))
	for _, item := range items {
		student, err := h.masterData.FindStudent(c.Request.Context(), orgID(c, h.orgID), item.StudentID)
		if err != nil {
			return nil, err
		}
		if _, ok := classes[student.SchoolClassID]; ok {
			filtered = append(filtered, item)
		}
	}
	return filtered, nil
}

func (h *Handler) currentCreator(c *gin.Context) (*uint64, string, error) {
	principal, ok := identity.PrincipalFromContext(c.Request.Context())
	if !ok || principal.Kind != identity.PrincipalKindUser || h.users == nil {
		return nil, "", nil
	}
	user, err := h.users.FindUserByID(c.Request.Context(), principal.SubjectID)
	if err != nil {
		return nil, "", err
	}
	if user.Status != identity.UserStatusActive {
		return nil, "", errors.New("wrongbook: current user is disabled")
	}
	return &principal.SubjectID, user.Nickname, nil
}

func (h *Handler) recordAudit(c *gin.Context, action, resourceType string, resourceID uint64) {
	var id *uint64
	if resourceID != 0 {
		id = &resourceID
	}
	auditmodule.RecordForContext(c.Request.Context(), h.audit, orgID(c, h.orgID), action, resourceType, id, "{}", c.GetHeader("X-Request-ID"))
}

func validateQuestionFields(studentID uint64, questionText, subject, answer, explanation, knowledgePoint, note, imageURL string) []response.ValidationDetail {
	details := make([]response.ValidationDetail, 0)
	if studentID == 0 {
		details = append(details, response.ValidationDetail{Field: "student_id", Reason: "required"})
	}
	if strings.TrimSpace(questionText) == "" {
		details = append(details, response.ValidationDetail{Field: "question_text", Reason: "required"})
	}
	if len([]rune(questionText)) > 2000 {
		details = append(details, response.ValidationDetail{Field: "question_text", Reason: "too_long"})
	}
	if len([]rune(subject)) > 64 {
		details = append(details, response.ValidationDetail{Field: "subject", Reason: "too_long"})
	}
	if len([]rune(answer)) > 1000 {
		details = append(details, response.ValidationDetail{Field: "answer_text", Reason: "too_long"})
	}
	if len([]rune(explanation)) > 2000 {
		details = append(details, response.ValidationDetail{Field: "explanation", Reason: "too_long"})
	}
	if len([]rune(knowledgePoint)) > 255 {
		details = append(details, response.ValidationDetail{Field: "knowledge_point", Reason: "too_long"})
	}
	if len([]rune(note)) > 500 {
		details = append(details, response.ValidationDetail{Field: "teacher_note", Reason: "too_long"})
	}
	if len([]rune(imageURL)) > 512 {
		details = append(details, response.ValidationDetail{Field: "source_image_url", Reason: "too_long"})
	}
	return details
}

func validatePaperFields(studentID uint64, title string, questionIDs []uint64) []response.ValidationDetail {
	details := make([]response.ValidationDetail, 0)
	if studentID == 0 {
		details = append(details, response.ValidationDetail{Field: "student_id", Reason: "required"})
	}
	if len(questionIDs) == 0 {
		details = append(details, response.ValidationDetail{Field: "question_ids", Reason: "required"})
	}
	if len(questionIDs) > 100 {
		details = append(details, response.ValidationDetail{Field: "question_ids", Reason: "too_many"})
	}
	if len([]rune(title)) > 128 {
		details = append(details, response.ValidationDetail{Field: "title", Reason: "too_long"})
	}
	return details
}

func extractQuestionCandidates(raw, subject string) []ExtractedQuestion {
	lines := normalizeQuestionLines(raw)
	if len(lines) == 0 {
		return nil
	}
	items := make([]ExtractedQuestion, 0, len(lines))
	for index, line := range lines {
		items = append(items, ExtractedQuestion{TempID: fmt.Sprintf("q%d", index+1), Subject: normalizeSubject(subject), QuestionText: line, Confidence: 0.82})
	}
	return items
}

var questionPrefixPattern = regexp.MustCompile(`^\s*(?:\d+[\.\)、)]|[（(]\d+[）)]|[一二三四五六七八九十]+[、.])\s*`)

func normalizeQuestionLines(raw string) []string {
	text := strings.ReplaceAll(strings.TrimSpace(raw), "\r\n", "\n")
	text = strings.ReplaceAll(text, "\r", "\n")
	if text == "" {
		return nil
	}
	parts := strings.Split(text, "\n")
	items := make([]string, 0, len(parts))
	var current strings.Builder
	flush := func() {
		value := strings.TrimSpace(current.String())
		if value != "" {
			items = append(items, value)
		}
		current.Reset()
	}
	for _, part := range parts {
		line := strings.TrimSpace(part)
		if line == "" {
			flush()
			continue
		}
		if questionPrefixPattern.MatchString(line) && current.Len() > 0 {
			flush()
		}
		if current.Len() > 0 {
			current.WriteString("\n")
		}
		current.WriteString(questionPrefixPattern.ReplaceAllString(line, ""))
	}
	flush()
	return items
}

func fallbackQuestionCandidates(subject string) []ExtractedQuestion {
	normalizedSubject := normalizeSubject(subject)
	return []ExtractedQuestion{
		{TempID: "q1", Subject: normalizedSubject, QuestionText: "图片已上传，请老师在这里校对第 1 道错题内容。", Confidence: 0.35},
		{TempID: "q2", Subject: normalizedSubject, QuestionText: "图片已上传，请老师在这里校对第 2 道错题内容。", Confidence: 0.35},
		{TempID: "q3", Subject: normalizedSubject, QuestionText: "图片已上传，请老师在这里校对第 3 道错题内容。", Confidence: 0.35},
	}
}

func normalizeSubject(value string) string {
	if strings.TrimSpace(value) == "" {
		return "综合"
	}
	return strings.TrimSpace(value)
}

func validQuestionStatus(status string) bool {
	return status == QuestionStatusActive || status == QuestionStatusMastered || status == QuestionStatusArchived
}

func canWriteWrongbook(c *gin.Context) bool {
	principal, ok := identity.PrincipalFromContext(c.Request.Context())
	if !ok {
		return true
	}
	if principal.Kind != identity.PrincipalKindUser || principal.Role == identity.UserRoleViewer {
		response.Error(c, response.Forbidden())
		return false
	}
	return true
}

func parsePathID(c *gin.Context, key string) (uint64, bool) {
	value, err := strconv.ParseUint(c.Param(key), 10, 64)
	if err != nil || value == 0 {
		response.Error(c, response.BadRequest(key+" 不合法", err))
		return 0, false
	}
	return value, true
}

func parseOptionalUint(c *gin.Context, key string) (uint64, bool) {
	value := strings.TrimSpace(c.Query(key))
	if value == "" {
		return 0, true
	}
	parsed, err := strconv.ParseUint(value, 10, 64)
	if err != nil {
		response.Error(c, response.ValidationFailed([]response.ValidationDetail{{Field: key, Reason: "invalid_value"}}))
		return 0, false
	}
	return parsed, true
}

func orgID(c *gin.Context, fallback uint64) uint64 {
	return identity.OrganizationIDFromContext(c.Request.Context(), fallback)
}

func toQuestionViews(items []Question) []questionView {
	out := make([]questionView, 0, len(items))
	for _, item := range items {
		out = append(out, toQuestionView(item))
	}
	return out
}

func toQuestionView(item Question) questionView {
	return questionView{ID: item.ID, StudentID: item.StudentID, StudentName: item.StudentName, Subject: item.Subject, QuestionText: item.QuestionText, AnswerText: item.AnswerText, Explanation: item.Explanation, KnowledgePoint: item.KnowledgePoint, SourceImageURL: item.SourceImageURL, SourceHomeworkTaskID: item.SourceHomeworkTaskID, TeacherNote: item.TeacherNote, Status: item.Status, CreatedByUserID: item.CreatedByUserID, CreatedByName: item.CreatedByName, LastReviewedAt: formatTime(item.LastReviewedAt), CreatedAt: item.CreatedAt.Format(time.RFC3339), UpdatedAt: item.UpdatedAt.Format(time.RFC3339)}
}

func toPaperViews(items []Paper) []paperView {
	out := make([]paperView, 0, len(items))
	for _, item := range items {
		out = append(out, toPaperView(item, false))
	}
	return out
}

func toPaperView(item Paper, includeQuestions bool) paperView {
	view := paperView{ID: item.ID, StudentID: item.StudentID, StudentName: item.StudentName, Title: item.Title, Source: item.Source, Status: item.Status, GeneratedByType: item.GeneratedByType, GeneratedByUserID: item.GeneratedByUserID, QuestionCount: item.QuestionCount, CreatedAt: item.CreatedAt.Format(time.RFC3339), UpdatedAt: item.UpdatedAt.Format(time.RFC3339)}
	if includeQuestions {
		view.Questions = toQuestionViews(item.Questions)
	}
	return view
}

func formatTime(value *time.Time) *string {
	if value == nil {
		return nil
	}
	formatted := value.Format(time.RFC3339)
	return &formatted
}

func respondStoreError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, ErrNotFound):
		response.Error(c, response.NotFound())
	case errors.Is(err, ErrInvalidStatus):
		response.Error(c, response.ValidationFailed([]response.ValidationDetail{{Field: "status", Reason: "invalid_value"}}))
	case errors.Is(err, ErrInvalidState), errors.Is(err, ErrConflict):
		response.Error(c, response.BadRequest("错题集数据不合法", err))
	default:
		response.Error(c, response.Internal(err))
	}
}

func respondAccessError(c *gin.Context, err error) {
	if errors.Is(err, ErrNotFound) {
		response.Error(c, response.BadRequest("当前教师没有负责该学生所在班级", err))
		return
	}
	response.Error(c, response.Internal(err))
}
