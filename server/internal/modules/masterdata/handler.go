package masterdata

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/chenbb0128/tuoguan-system-server/internal/modules/identity"
	"github.com/chenbb0128/tuoguan-system-server/internal/transport/httpapi/request"
	"github.com/chenbb0128/tuoguan-system-server/internal/transport/httpapi/response"
)

type Handler struct {
	store Store
	orgID uint64
}

type schoolClassScopeContextKey struct{}

func WithSchoolClassScope(ctx context.Context, classIDs []uint64) context.Context {
	allowed := make(map[uint64]struct{}, len(classIDs))
	for _, classID := range classIDs {
		if classID != 0 {
			allowed[classID] = struct{}{}
		}
	}
	return context.WithValue(ctx, schoolClassScopeContextKey{}, allowed)
}

func SchoolClassScopeFromContext(ctx context.Context) (map[uint64]struct{}, bool) {
	allowed, ok := ctx.Value(schoolClassScopeContextKey{}).(map[uint64]struct{})
	return allowed, ok
}

func NewHandler(store Store) *Handler {
	return &Handler{store: store, orgID: DefaultOrganizationID}
}

func (h *Handler) RegisterRoutes(api *gin.RouterGroup) {
	h.RegisterReadRoutes(api)
	h.RegisterWriteRoutes(api)
}

func (h *Handler) RegisterReadRoutes(api *gin.RouterGroup) {
	api.GET("/summary", h.summary)
	api.GET("/schools", h.listSchools)
	api.GET("/academic-terms", h.listAcademicTerms)
	api.GET("/school-classes", h.listSchoolClasses)
	api.GET("/care-classes", h.listCareClasses)
	api.GET("/students", h.listStudents)
}

func (h *Handler) RegisterWriteRoutes(api *gin.RouterGroup) {
	api.POST("/schools", h.createSchool)
	api.POST("/academic-terms", h.createAcademicTerm)
	api.POST("/school-classes", h.createSchoolClass)
	api.POST("/care-classes", h.createCareClass)
	api.POST("/students", h.createStudent)
	api.POST("/students/profile", h.createStudentProfile)
	api.PUT("/students/:id", h.updateStudent)
	api.PUT("/students/:id/profile", h.updateStudentProfile)
}

type listResponse[T any] struct {
	Items []T `json:"items"`
	Total int `json:"total"`
}

type schoolView struct {
	ID           uint64 `json:"id"`
	Name         string `json:"name"`
	Address      string `json:"address"`
	ContactPhone string `json:"contact_phone"`
	Status       string `json:"status"`
	CreatedAt    string `json:"created_at"`
	UpdatedAt    string `json:"updated_at"`
}

type academicTermView struct {
	ID        uint64 `json:"id"`
	Name      string `json:"name"`
	StartsOn  string `json:"starts_on"`
	EndsOn    string `json:"ends_on"`
	IsCurrent bool   `json:"is_current"`
	Status    string `json:"status"`
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
}

type schoolClassView struct {
	ID       uint64 `json:"id"`
	SchoolID uint64 `json:"school_id"`
	TermID   uint64 `json:"term_id"`
	Grade    string `json:"grade"`
	Name     string `json:"name"`
	Status   string `json:"status"`
}

type careClassView struct {
	ID       uint64 `json:"id"`
	Name     string `json:"name"`
	Capacity uint32 `json:"capacity"`
	Status   string `json:"status"`
}

type studentView struct {
	ID               uint64  `json:"id"`
	SchoolID         uint64  `json:"school_id"`
	TermID           uint64  `json:"term_id"`
	SchoolClassID    uint64  `json:"school_class_id"`
	CareClassID      *uint64 `json:"care_class_id,omitempty"`
	Name             string  `json:"name"`
	Gender           string  `json:"gender"`
	BirthDate        *string `json:"birth_date,omitempty"`
	StudentNo        string  `json:"student_no"`
	GuardianPhone    string  `json:"guardian_phone"`
	EmergencyContact string  `json:"emergency_contact"`
	EmergencyPhone   string  `json:"emergency_phone"`
	Status           string  `json:"status"`
	Notes            string  `json:"notes"`
	CreatedAt        string  `json:"created_at"`
	UpdatedAt        string  `json:"updated_at"`
}

type createSchoolRequest struct {
	Name         string `json:"name"`
	Address      string `json:"address"`
	ContactPhone string `json:"contact_phone"`
}

func (r createSchoolRequest) Validate() []response.ValidationDetail {
	if strings.TrimSpace(r.Name) == "" {
		return []response.ValidationDetail{{Field: "name", Reason: "required"}}
	}
	return nil
}

type createAcademicTermRequest struct {
	Name      string `json:"name"`
	StartsOn  string `json:"starts_on"`
	EndsOn    string `json:"ends_on"`
	IsCurrent bool   `json:"is_current"`
}

func (r createAcademicTermRequest) Validate() []response.ValidationDetail {
	details := make([]response.ValidationDetail, 0, 3)
	if strings.TrimSpace(r.Name) == "" {
		details = append(details, response.ValidationDetail{Field: "name", Reason: "required"})
	}
	if _, err := parseDate(r.StartsOn); err != nil {
		details = append(details, response.ValidationDetail{Field: "starts_on", Reason: "date_format"})
	}
	if _, err := parseDate(r.EndsOn); err != nil {
		details = append(details, response.ValidationDetail{Field: "ends_on", Reason: "date_format"})
	}
	return details
}

type createSchoolClassRequest struct {
	SchoolID uint64 `json:"school_id"`
	TermID   uint64 `json:"term_id"`
	Grade    string `json:"grade"`
	Name     string `json:"name"`
}

func (r createSchoolClassRequest) Validate() []response.ValidationDetail {
	details := make([]response.ValidationDetail, 0, 4)
	if r.SchoolID == 0 {
		details = append(details, response.ValidationDetail{Field: "school_id", Reason: "required"})
	}
	if r.TermID == 0 {
		details = append(details, response.ValidationDetail{Field: "term_id", Reason: "required"})
	}
	if strings.TrimSpace(r.Grade) == "" {
		details = append(details, response.ValidationDetail{Field: "grade", Reason: "required"})
	}
	if strings.TrimSpace(r.Name) == "" {
		details = append(details, response.ValidationDetail{Field: "name", Reason: "required"})
	}
	return details
}

type createCareClassRequest struct {
	Name     string `json:"name"`
	Capacity uint32 `json:"capacity"`
}

func (r createCareClassRequest) Validate() []response.ValidationDetail {
	if strings.TrimSpace(r.Name) == "" {
		return []response.ValidationDetail{{Field: "name", Reason: "required"}}
	}
	return nil
}

type studentRequest struct {
	SchoolID         uint64  `json:"school_id"`
	TermID           uint64  `json:"term_id"`
	SchoolClassID    uint64  `json:"school_class_id"`
	CareClassID      *uint64 `json:"care_class_id"`
	Name             string  `json:"name"`
	Gender           string  `json:"gender"`
	BirthDate        string  `json:"birth_date"`
	StudentNo        string  `json:"student_no"`
	GuardianPhone    string  `json:"guardian_phone"`
	EmergencyContact string  `json:"emergency_contact"`
	EmergencyPhone   string  `json:"emergency_phone"`
	Status           string  `json:"status"`
	Notes            string  `json:"notes"`
}

// studentProfileRequest is the user-facing form. The IDs remain an internal
// implementation detail so daily archive maintenance can be done from one
// student form while the service still keeps structured relationships.
type studentProfileRequest struct {
	SchoolName       string `json:"school_name"`
	TermID           uint64 `json:"term_id"`
	TermName         string `json:"term_name"`
	Grade            string `json:"grade"`
	ClassName        string `json:"class_name"`
	CareClassName    string `json:"care_class_name"`
	Name             string `json:"name"`
	Gender           string `json:"gender"`
	BirthDate        string `json:"birth_date"`
	StudentNo        string `json:"student_no"`
	GuardianPhone    string `json:"guardian_phone"`
	EmergencyContact string `json:"emergency_contact"`
	EmergencyPhone   string `json:"emergency_phone"`
	Status           string `json:"status"`
	Notes            string `json:"notes"`
}

func (r studentProfileRequest) Validate() []response.ValidationDetail {
	details := make([]response.ValidationDetail, 0, 7)
	for field, value := range map[string]string{
		"school_name": r.SchoolName,
		"grade":       r.Grade,
		"class_name":  r.ClassName,
		"name":        r.Name,
	} {
		if strings.TrimSpace(value) == "" {
			details = append(details, response.ValidationDetail{Field: field, Reason: "required"})
		}
	}
	if r.Gender != "" && r.Gender != "unknown" && r.Gender != "male" && r.Gender != "female" {
		details = append(details, response.ValidationDetail{Field: "gender", Reason: "invalid_value"})
	}
	if r.Status != "" && r.Status != "active" && r.Status != "inactive" {
		details = append(details, response.ValidationDetail{Field: "status", Reason: "invalid_value"})
	}
	if strings.TrimSpace(r.BirthDate) != "" {
		if _, err := parseDate(r.BirthDate); err != nil {
			details = append(details, response.ValidationDetail{Field: "birth_date", Reason: "date_format"})
		}
	}
	return details
}

func (r studentRequest) Validate() []response.ValidationDetail {
	details := make([]response.ValidationDetail, 0, 6)
	if r.SchoolID == 0 {
		details = append(details, response.ValidationDetail{Field: "school_id", Reason: "required"})
	}
	if r.TermID == 0 {
		details = append(details, response.ValidationDetail{Field: "term_id", Reason: "required"})
	}
	if r.SchoolClassID == 0 {
		details = append(details, response.ValidationDetail{Field: "school_class_id", Reason: "required"})
	}
	if strings.TrimSpace(r.Name) == "" {
		details = append(details, response.ValidationDetail{Field: "name", Reason: "required"})
	}
	if r.Gender != "" && r.Gender != "unknown" && r.Gender != "male" && r.Gender != "female" {
		details = append(details, response.ValidationDetail{Field: "gender", Reason: "invalid_value"})
	}
	if r.Status != "" && r.Status != "active" && r.Status != "inactive" {
		details = append(details, response.ValidationDetail{Field: "status", Reason: "invalid_value"})
	}
	if strings.TrimSpace(r.BirthDate) != "" {
		if _, err := parseDate(r.BirthDate); err != nil {
			details = append(details, response.ValidationDetail{Field: "birth_date", Reason: "date_format"})
		}
	}
	return details
}

func (h *Handler) summary(c *gin.Context) {
	schools, err := h.store.ListSchools(c.Request.Context(), identity.OrganizationIDFromContext(c.Request.Context(), h.orgID))
	if err != nil {
		respondStoreError(c, err)
		return
	}
	terms, err := h.store.ListAcademicTerms(c.Request.Context(), identity.OrganizationIDFromContext(c.Request.Context(), h.orgID))
	if err != nil {
		respondStoreError(c, err)
		return
	}
	schoolClasses, err := h.store.ListSchoolClasses(c.Request.Context(), identity.OrganizationIDFromContext(c.Request.Context(), h.orgID))
	if err != nil {
		respondStoreError(c, err)
		return
	}
	careClasses, err := h.store.ListCareClasses(c.Request.Context(), identity.OrganizationIDFromContext(c.Request.Context(), h.orgID))
	if err != nil {
		respondStoreError(c, err)
		return
	}
	students, err := h.store.ListStudents(c.Request.Context(), identity.OrganizationIDFromContext(c.Request.Context(), h.orgID))
	if err != nil {
		respondStoreError(c, err)
		return
	}
	response.OK(c, gin.H{"schools": len(schools), "academic_terms": len(terms), "school_classes": len(schoolClasses), "care_classes": len(careClasses), "students": len(students)})
}

func (h *Handler) listSchools(c *gin.Context) {
	items, err := h.store.ListSchools(c.Request.Context(), identity.OrganizationIDFromContext(c.Request.Context(), h.orgID))
	if err != nil {
		respondStoreError(c, err)
		return
	}
	out := make([]schoolView, 0, len(items))
	for _, item := range items {
		out = append(out, toSchoolView(item))
	}
	response.OK(c, listResponse[schoolView]{Items: out, Total: len(out)})
}

func (h *Handler) createSchool(c *gin.Context) {
	var req createSchoolRequest
	if err := request.BindJSON(c, &req); err != nil {
		response.Error(c, err)
		return
	}
	item, err := h.store.CreateSchool(c.Request.Context(), identity.OrganizationIDFromContext(c.Request.Context(), h.orgID), CreateSchoolParams{Name: strings.TrimSpace(req.Name), Address: strings.TrimSpace(req.Address), ContactPhone: strings.TrimSpace(req.ContactPhone)})
	if err != nil {
		respondStoreError(c, err)
		return
	}
	response.Created(c, "/api/v1/schools/"+strconv.FormatUint(item.ID, 10), toSchoolView(item))
}

func (h *Handler) listAcademicTerms(c *gin.Context) {
	items, err := h.store.ListAcademicTerms(c.Request.Context(), identity.OrganizationIDFromContext(c.Request.Context(), h.orgID))
	if err != nil {
		respondStoreError(c, err)
		return
	}
	out := make([]academicTermView, 0, len(items))
	for _, item := range items {
		out = append(out, toAcademicTermView(item))
	}
	response.OK(c, listResponse[academicTermView]{Items: out, Total: len(out)})
}

func (h *Handler) createAcademicTerm(c *gin.Context) {
	var req createAcademicTermRequest
	if err := request.BindJSON(c, &req); err != nil {
		response.Error(c, err)
		return
	}
	startsOn, _ := parseDate(req.StartsOn)
	endsOn, _ := parseDate(req.EndsOn)
	if startsOn.After(endsOn) {
		response.Error(c, response.ValidationFailed([]response.ValidationDetail{{Field: "ends_on", Reason: "must_not_be_before_starts_on"}}))
		return
	}
	item, err := h.store.CreateAcademicTerm(c.Request.Context(), identity.OrganizationIDFromContext(c.Request.Context(), h.orgID), CreateAcademicTermParams{Name: strings.TrimSpace(req.Name), StartsOn: startsOn, EndsOn: endsOn, IsCurrent: req.IsCurrent})
	if err != nil {
		respondStoreError(c, err)
		return
	}
	response.Created(c, "/api/v1/academic-terms/"+strconv.FormatUint(item.ID, 10), toAcademicTermView(item))
}

func (h *Handler) listSchoolClasses(c *gin.Context) {
	items, err := h.store.ListSchoolClasses(c.Request.Context(), identity.OrganizationIDFromContext(c.Request.Context(), h.orgID))
	if err != nil {
		respondStoreError(c, err)
		return
	}
	if allowed, scoped := SchoolClassScopeFromContext(c.Request.Context()); scoped {
		filtered := make([]SchoolClass, 0, len(items))
		for _, item := range items {
			if _, ok := allowed[item.ID]; ok {
				filtered = append(filtered, item)
			}
		}
		items = filtered
	}
	out := make([]schoolClassView, 0, len(items))
	for _, item := range items {
		out = append(out, toSchoolClassView(item))
	}
	response.OK(c, listResponse[schoolClassView]{Items: out, Total: len(out)})
}

func (h *Handler) createSchoolClass(c *gin.Context) {
	var req createSchoolClassRequest
	if err := request.BindJSON(c, &req); err != nil {
		response.Error(c, err)
		return
	}
	item, err := h.store.CreateSchoolClass(c.Request.Context(), identity.OrganizationIDFromContext(c.Request.Context(), h.orgID), CreateSchoolClassParams{SchoolID: req.SchoolID, TermID: req.TermID, Grade: strings.TrimSpace(req.Grade), Name: strings.TrimSpace(req.Name)})
	if err != nil {
		respondStoreError(c, err)
		return
	}
	response.Created(c, "/api/v1/school-classes/"+strconv.FormatUint(item.ID, 10), toSchoolClassView(item))
}

func (h *Handler) listCareClasses(c *gin.Context) {
	items, err := h.store.ListCareClasses(c.Request.Context(), identity.OrganizationIDFromContext(c.Request.Context(), h.orgID))
	if err != nil {
		respondStoreError(c, err)
		return
	}
	out := make([]careClassView, 0, len(items))
	for _, item := range items {
		out = append(out, toCareClassView(item))
	}
	response.OK(c, listResponse[careClassView]{Items: out, Total: len(out)})
}

func (h *Handler) createCareClass(c *gin.Context) {
	var req createCareClassRequest
	if err := request.BindJSON(c, &req); err != nil {
		response.Error(c, err)
		return
	}
	item, err := h.store.CreateCareClass(c.Request.Context(), identity.OrganizationIDFromContext(c.Request.Context(), h.orgID), CreateCareClassParams{Name: strings.TrimSpace(req.Name), Capacity: req.Capacity})
	if err != nil {
		respondStoreError(c, err)
		return
	}
	response.Created(c, "/api/v1/care-classes/"+strconv.FormatUint(item.ID, 10), toCareClassView(item))
}

func (h *Handler) listStudents(c *gin.Context) {
	items, err := h.store.ListStudents(c.Request.Context(), identity.OrganizationIDFromContext(c.Request.Context(), h.orgID))
	if err != nil {
		respondStoreError(c, err)
		return
	}
	if allowed, scoped := SchoolClassScopeFromContext(c.Request.Context()); scoped {
		filtered := make([]Student, 0, len(items))
		for _, item := range items {
			if _, ok := allowed[item.SchoolClassID]; ok {
				filtered = append(filtered, item)
			}
		}
		items = filtered
	}
	if value := c.Query("school_id"); value != "" {
		items = filterStudentsByID(items, value, func(item Student) uint64 { return item.SchoolID })
	}
	if value := c.Query("term_id"); value != "" {
		items = filterStudentsByID(items, value, func(item Student) uint64 { return item.TermID })
	}
	if value := c.Query("school_class_id"); value != "" {
		items = filterStudentsByID(items, value, func(item Student) uint64 { return item.SchoolClassID })
	}
	if value := c.Query("care_class_id"); value != "" {
		items = filterStudentsByID(items, value, func(item Student) uint64 {
			if item.CareClassID == nil {
				return 0
			}
			return *item.CareClassID
		})
	}
	if value := strings.TrimSpace(c.Query("grade")); value != "" {
		classes, err := h.store.ListSchoolClasses(c.Request.Context(), identity.OrganizationIDFromContext(c.Request.Context(), h.orgID))
		if err != nil {
			respondStoreError(c, err)
			return
		}
		classIDs := make(map[uint64]struct{})
		for _, item := range classes {
			if strings.EqualFold(strings.TrimSpace(item.Grade), value) {
				classIDs[item.ID] = struct{}{}
			}
		}
		items = filterStudentsByClassIDs(items, classIDs)
	}
	if value := strings.TrimSpace(c.Query("status")); value != "" {
		items = filterStudentsByStatus(items, value)
	}
	if keyword := strings.TrimSpace(c.Query("keyword")); keyword != "" {
		items = filterStudentsByKeyword(items, keyword)
	}
	out := make([]studentView, 0, len(items))
	for _, item := range items {
		out = append(out, toStudentView(item))
	}
	response.OK(c, listResponse[studentView]{Items: out, Total: len(out)})
}

func (h *Handler) createStudent(c *gin.Context) {
	var req studentRequest
	if err := request.BindJSON(c, &req); err != nil {
		response.Error(c, err)
		return
	}
	item, err := h.store.CreateStudent(c.Request.Context(), identity.OrganizationIDFromContext(c.Request.Context(), h.orgID), toCreateStudentParams(req))
	if err != nil {
		respondStoreError(c, err)
		return
	}
	response.Created(c, "/api/v1/students/"+strconv.FormatUint(item.ID, 10), toStudentView(item))
}

func (h *Handler) createStudentProfile(c *gin.Context) {
	var req studentProfileRequest
	if err := request.BindJSON(c, &req); err != nil {
		response.Error(c, err)
		return
	}
	params, err := h.resolveStudentProfile(c.Request.Context(), req, 0)
	if err != nil {
		respondStoreError(c, err)
		return
	}
	item, err := h.store.CreateStudent(c.Request.Context(), identity.OrganizationIDFromContext(c.Request.Context(), h.orgID), params)
	if err != nil {
		respondStoreError(c, err)
		return
	}
	response.Created(c, "/api/v1/students/"+strconv.FormatUint(item.ID, 10), toStudentView(item))
}

func (h *Handler) updateStudent(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || id == 0 {
		response.Error(c, response.BadRequest("学生 ID 不合法", err))
		return
	}
	var req studentRequest
	if err := request.BindJSON(c, &req); err != nil {
		response.Error(c, err)
		return
	}
	params := toUpdateStudentParams(req)
	params.ID = id
	if params.Status == "" {
		params.Status = "active"
	}
	item, err := h.store.UpdateStudent(c.Request.Context(), identity.OrganizationIDFromContext(c.Request.Context(), h.orgID), params)
	if err != nil {
		respondStoreError(c, err)
		return
	}
	response.OK(c, toStudentView(item))
}

func (h *Handler) updateStudentProfile(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || id == 0 {
		response.Error(c, response.BadRequest("学生 ID 不合法", err))
		return
	}
	var req studentProfileRequest
	if err := request.BindJSON(c, &req); err != nil {
		response.Error(c, err)
		return
	}
	existing, err := h.store.FindStudent(c.Request.Context(), identity.OrganizationIDFromContext(c.Request.Context(), h.orgID), id)
	if err != nil {
		respondStoreError(c, err)
		return
	}
	params, err := h.resolveStudentProfile(c.Request.Context(), req, existing.TermID)
	if err != nil {
		respondStoreError(c, err)
		return
	}
	update := UpdateStudentParams{
		ID:               id,
		SchoolID:         params.SchoolID,
		TermID:           params.TermID,
		SchoolClassID:    params.SchoolClassID,
		CareClassID:      params.CareClassID,
		Name:             params.Name,
		Gender:           params.Gender,
		BirthDate:        params.BirthDate,
		StudentNo:        params.StudentNo,
		GuardianPhone:    params.GuardianPhone,
		EmergencyContact: params.EmergencyContact,
		EmergencyPhone:   params.EmergencyPhone,
		Status:           defaultString(req.Status, existing.Status),
		Notes:            params.Notes,
	}
	item, err := h.store.UpdateStudent(c.Request.Context(), identity.OrganizationIDFromContext(c.Request.Context(), h.orgID), update)
	if err != nil {
		respondStoreError(c, err)
		return
	}
	response.OK(c, toStudentView(item))
}

func (h *Handler) resolveStudentProfile(ctx context.Context, req studentProfileRequest, fallbackTermID uint64) (CreateStudentParams, error) {
	organizationID := identity.OrganizationIDFromContext(ctx, h.orgID)
	schools, err := h.store.ListSchools(ctx, organizationID)
	if err != nil {
		return CreateStudentParams{}, err
	}
	schoolName := strings.TrimSpace(req.SchoolName)
	var school *School
	for index := range schools {
		if strings.EqualFold(strings.TrimSpace(schools[index].Name), schoolName) && schools[index].Status != "disabled" {
			school = &schools[index]
			break
		}
	}
	if school == nil {
		created, createErr := h.store.CreateSchool(ctx, organizationID, CreateSchoolParams{Name: schoolName})
		if createErr != nil && !errors.Is(createErr, ErrConflict) {
			return CreateStudentParams{}, createErr
		}
		if createErr == nil {
			school = &created
		} else {
			school, err = findSchool(ctx, h.store, organizationID, schoolName)
			if err != nil {
				return CreateStudentParams{}, err
			}
		}
	}

	terms, err := h.store.ListAcademicTerms(ctx, organizationID)
	if err != nil {
		return CreateStudentParams{}, err
	}
	term, err := resolveTerm(ctx, h.store, organizationID, terms, req, fallbackTermID)
	if err != nil {
		return CreateStudentParams{}, err
	}

	classes, err := h.store.ListSchoolClasses(ctx, organizationID)
	if err != nil {
		return CreateStudentParams{}, err
	}
	grade := strings.TrimSpace(req.Grade)
	className := strings.TrimSpace(req.ClassName)
	var schoolClass *SchoolClass
	for index := range classes {
		item := &classes[index]
		if item.SchoolID == school.ID && item.TermID == term.ID && strings.EqualFold(strings.TrimSpace(item.Grade), grade) && strings.EqualFold(strings.TrimSpace(item.Name), className) && item.Status != "disabled" {
			schoolClass = item
			break
		}
	}
	if schoolClass == nil {
		created, createErr := h.store.CreateSchoolClass(ctx, organizationID, CreateSchoolClassParams{SchoolID: school.ID, TermID: term.ID, Grade: grade, Name: className})
		if createErr != nil && !errors.Is(createErr, ErrConflict) {
			return CreateStudentParams{}, createErr
		}
		if createErr == nil {
			schoolClass = &created
		} else {
			schoolClass, err = findSchoolClass(ctx, h.store, organizationID, school.ID, term.ID, grade, className)
			if err != nil {
				return CreateStudentParams{}, err
			}
		}
	}

	var careClassID *uint64
	careClassName := strings.TrimSpace(req.CareClassName)
	if careClassName != "" {
		careClasses, listErr := h.store.ListCareClasses(ctx, organizationID)
		if listErr != nil {
			return CreateStudentParams{}, listErr
		}
		var careClass *CareClass
		for index := range careClasses {
			if strings.EqualFold(strings.TrimSpace(careClasses[index].Name), careClassName) && careClasses[index].Status != "disabled" {
				careClass = &careClasses[index]
				break
			}
		}
		if careClass == nil {
			created, createErr := h.store.CreateCareClass(ctx, organizationID, CreateCareClassParams{Name: careClassName})
			if createErr != nil && !errors.Is(createErr, ErrConflict) {
				return CreateStudentParams{}, createErr
			}
			if createErr == nil {
				careClass = &created
			} else {
				careClass, err = findCareClass(ctx, h.store, organizationID, careClassName)
				if err != nil {
					return CreateStudentParams{}, err
				}
			}
		}
		value := careClass.ID
		careClassID = &value
	}

	return CreateStudentParams{
		SchoolID:         school.ID,
		TermID:           term.ID,
		SchoolClassID:    schoolClass.ID,
		CareClassID:      careClassID,
		Name:             strings.TrimSpace(req.Name),
		Gender:           defaultString(req.Gender, "unknown"),
		BirthDate:        optionalDate(req.BirthDate),
		StudentNo:        strings.TrimSpace(req.StudentNo),
		GuardianPhone:    strings.TrimSpace(req.GuardianPhone),
		EmergencyContact: strings.TrimSpace(req.EmergencyContact),
		EmergencyPhone:   strings.TrimSpace(req.EmergencyPhone),
		Notes:            strings.TrimSpace(req.Notes),
	}, nil
}

func resolveTerm(ctx context.Context, store Store, orgID uint64, terms []AcademicTerm, req studentProfileRequest, fallbackTermID uint64) (*AcademicTerm, error) {
	if req.TermID != 0 {
		for index := range terms {
			if terms[index].ID == req.TermID && terms[index].Status != "disabled" {
				return &terms[index], nil
			}
		}
		return nil, fmt.Errorf("%w: academic term %d", ErrNotFound, req.TermID)
	}
	if fallbackTermID != 0 && strings.TrimSpace(req.TermName) == "" {
		for index := range terms {
			if terms[index].ID == fallbackTermID && terms[index].Status != "disabled" {
				return &terms[index], nil
			}
		}
	}
	termName := strings.TrimSpace(req.TermName)
	if termName != "" {
		for index := range terms {
			if strings.EqualFold(strings.TrimSpace(terms[index].Name), termName) && terms[index].Status != "disabled" {
				return &terms[index], nil
			}
		}
		params := defaultAcademicTermParams(time.Now().UTC())
		params.Name = termName
		created, err := store.CreateAcademicTerm(ctx, orgID, params)
		if err == nil {
			return &created, nil
		}
		if !errors.Is(err, ErrConflict) {
			return nil, err
		}
		return findAcademicTerm(ctx, store, orgID, termName)
	}
	for index := range terms {
		if terms[index].IsCurrent && terms[index].Status != "disabled" {
			return &terms[index], nil
		}
	}
	params := defaultAcademicTermParams(time.Now().UTC())
	created, err := store.CreateAcademicTerm(ctx, orgID, params)
	if err == nil {
		return &created, nil
	}
	if !errors.Is(err, ErrConflict) {
		return nil, err
	}
	return findAcademicTerm(ctx, store, orgID, params.Name)
}

func defaultAcademicTermParams(now time.Time) CreateAcademicTermParams {
	if now.Month() >= time.August {
		return CreateAcademicTermParams{Name: fmt.Sprintf("%d-%d学年第一学期", now.Year(), now.Year()+1), StartsOn: time.Date(now.Year(), time.August, 1, 0, 0, 0, 0, time.UTC), EndsOn: time.Date(now.Year()+1, time.January, 31, 0, 0, 0, 0, time.UTC), IsCurrent: true}
	}
	return CreateAcademicTermParams{Name: fmt.Sprintf("%d-%d学年第二学期", now.Year()-1, now.Year()), StartsOn: time.Date(now.Year(), time.February, 1, 0, 0, 0, 0, time.UTC), EndsOn: time.Date(now.Year(), time.July, 31, 0, 0, 0, 0, time.UTC), IsCurrent: true}
}

func findSchool(ctx context.Context, store Store, orgID uint64, name string) (*School, error) {
	items, err := store.ListSchools(ctx, orgID)
	if err != nil {
		return nil, err
	}
	for index := range items {
		if strings.EqualFold(strings.TrimSpace(items[index].Name), strings.TrimSpace(name)) && items[index].Status != "disabled" {
			return &items[index], nil
		}
	}
	return nil, fmt.Errorf("%w: school %q", ErrNotFound, name)
}

func findAcademicTerm(ctx context.Context, store Store, orgID uint64, name string) (*AcademicTerm, error) {
	items, err := store.ListAcademicTerms(ctx, orgID)
	if err != nil {
		return nil, err
	}
	for index := range items {
		if strings.EqualFold(strings.TrimSpace(items[index].Name), strings.TrimSpace(name)) && items[index].Status != "disabled" {
			return &items[index], nil
		}
	}
	return nil, fmt.Errorf("%w: academic term %q", ErrNotFound, name)
}

func findSchoolClass(ctx context.Context, store Store, orgID, schoolID, termID uint64, grade, name string) (*SchoolClass, error) {
	items, err := store.ListSchoolClasses(ctx, orgID)
	if err != nil {
		return nil, err
	}
	for index := range items {
		if items[index].SchoolID == schoolID && items[index].TermID == termID && strings.EqualFold(strings.TrimSpace(items[index].Grade), strings.TrimSpace(grade)) && strings.EqualFold(strings.TrimSpace(items[index].Name), strings.TrimSpace(name)) && items[index].Status != "disabled" {
			return &items[index], nil
		}
	}
	return nil, fmt.Errorf("%w: school class %q %q", ErrNotFound, grade, name)
}

func findCareClass(ctx context.Context, store Store, orgID uint64, name string) (*CareClass, error) {
	items, err := store.ListCareClasses(ctx, orgID)
	if err != nil {
		return nil, err
	}
	for index := range items {
		if strings.EqualFold(strings.TrimSpace(items[index].Name), strings.TrimSpace(name)) && items[index].Status != "disabled" {
			return &items[index], nil
		}
	}
	return nil, fmt.Errorf("%w: care class %q", ErrNotFound, name)
}

func respondStoreError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, ErrConflict):
		response.Error(c, response.BadRequest("记录已存在", err))
	case errors.Is(err, ErrNotFound):
		response.Error(c, response.NotFound())
	default:
		response.Error(c, response.Internal(err))
	}
}

func parseDate(value string) (time.Time, error) {
	return time.ParseInLocation("2006-01-02", strings.TrimSpace(value), time.UTC)
}

func toSchoolView(item School) schoolView {
	return schoolView{ID: item.ID, Name: item.Name, Address: item.Address, ContactPhone: item.ContactPhone, Status: item.Status, CreatedAt: item.CreatedAt.Format(time.RFC3339), UpdatedAt: item.UpdatedAt.Format(time.RFC3339)}
}
func toAcademicTermView(item AcademicTerm) academicTermView {
	return academicTermView{ID: item.ID, Name: item.Name, StartsOn: item.StartsOn.Format("2006-01-02"), EndsOn: item.EndsOn.Format("2006-01-02"), IsCurrent: item.IsCurrent, Status: item.Status, CreatedAt: item.CreatedAt.Format(time.RFC3339), UpdatedAt: item.UpdatedAt.Format(time.RFC3339)}
}
func toSchoolClassView(item SchoolClass) schoolClassView {
	return schoolClassView{ID: item.ID, SchoolID: item.SchoolID, TermID: item.TermID, Grade: item.Grade, Name: item.Name, Status: item.Status}
}
func toCareClassView(item CareClass) careClassView {
	return careClassView{ID: item.ID, Name: item.Name, Capacity: item.Capacity, Status: item.Status}
}
func toStudentView(item Student) studentView {
	var birthDate *string
	if item.BirthDate != nil {
		value := item.BirthDate.Format("2006-01-02")
		birthDate = &value
	}
	return studentView{ID: item.ID, SchoolID: item.SchoolID, TermID: item.TermID, SchoolClassID: item.SchoolClassID, CareClassID: item.CareClassID, Name: item.Name, Gender: item.Gender, BirthDate: birthDate, StudentNo: item.StudentNo, GuardianPhone: item.GuardianPhone, EmergencyContact: item.EmergencyContact, EmergencyPhone: item.EmergencyPhone, Status: item.Status, Notes: item.Notes, CreatedAt: item.CreatedAt.Format(time.RFC3339), UpdatedAt: item.UpdatedAt.Format(time.RFC3339)}
}
func toCreateStudentParams(req studentRequest) CreateStudentParams {
	return CreateStudentParams{SchoolID: req.SchoolID, TermID: req.TermID, SchoolClassID: req.SchoolClassID, CareClassID: req.CareClassID, Name: strings.TrimSpace(req.Name), Gender: defaultString(req.Gender, "unknown"), BirthDate: optionalDate(req.BirthDate), StudentNo: strings.TrimSpace(req.StudentNo), GuardianPhone: strings.TrimSpace(req.GuardianPhone), EmergencyContact: strings.TrimSpace(req.EmergencyContact), EmergencyPhone: strings.TrimSpace(req.EmergencyPhone), Notes: strings.TrimSpace(req.Notes)}
}
func toUpdateStudentParams(req studentRequest) UpdateStudentParams {
	return UpdateStudentParams{SchoolID: req.SchoolID, TermID: req.TermID, SchoolClassID: req.SchoolClassID, CareClassID: req.CareClassID, Name: strings.TrimSpace(req.Name), Gender: defaultString(req.Gender, "unknown"), BirthDate: optionalDate(req.BirthDate), StudentNo: strings.TrimSpace(req.StudentNo), GuardianPhone: strings.TrimSpace(req.GuardianPhone), EmergencyContact: strings.TrimSpace(req.EmergencyContact), EmergencyPhone: strings.TrimSpace(req.EmergencyPhone), Status: defaultString(req.Status, "active"), Notes: strings.TrimSpace(req.Notes)}
}
func optionalDate(value string) *time.Time {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	parsed, _ := parseDate(value)
	return &parsed
}
func defaultString(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return strings.TrimSpace(value)
}
func filterStudentsByID(items []Student, value string, get func(Student) uint64) []Student {
	id, err := strconv.ParseUint(value, 10, 64)
	if err != nil {
		return nil
	}
	out := make([]Student, 0, len(items))
	for _, item := range items {
		if get(item) == id {
			out = append(out, item)
		}
	}
	return out
}
func filterStudentsByKeyword(items []Student, keyword string) []Student {
	keyword = strings.ToLower(keyword)
	out := make([]Student, 0, len(items))
	for _, item := range items {
		if strings.Contains(strings.ToLower(item.Name), keyword) || strings.Contains(strings.ToLower(item.StudentNo), keyword) {
			out = append(out, item)
		}
	}
	return out
}

func filterStudentsByClassIDs(items []Student, classIDs map[uint64]struct{}) []Student {
	out := make([]Student, 0, len(items))
	for _, item := range items {
		if _, ok := classIDs[item.SchoolClassID]; ok {
			out = append(out, item)
		}
	}
	return out
}

func filterStudentsByStatus(items []Student, status string) []Student {
	out := make([]Student, 0, len(items))
	for _, item := range items {
		if strings.EqualFold(item.Status, status) {
			out = append(out, item)
		}
	}
	return out
}
