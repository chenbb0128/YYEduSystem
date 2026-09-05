package masterdata

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"sync"
	"time"
)

// MemoryStore keeps the API useful when local infrastructure has not been started yet.
// It is deliberately only a development fallback; configured MySQL always takes precedence.
type MemoryStore struct {
	mu            sync.RWMutex
	nextID        uint64
	schools       []School
	terms         []AcademicTerm
	schoolClasses []SchoolClass
	careClasses   []CareClass
	students      []Student
}

func NewMemoryStore() *MemoryStore {
	return &MemoryStore{nextID: 1}
}

func (s *MemoryStore) newID() uint64 {
	id := s.nextID
	s.nextID++
	return id
}

func (s *MemoryStore) ListSchools(context.Context, uint64) ([]School, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return slices.Clone(s.schools), nil
}

func (s *MemoryStore) CreateSchool(_ context.Context, orgID uint64, params CreateSchoolParams) (School, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, item := range s.schools {
		if item.OrganizationID == orgID && strings.EqualFold(item.Name, params.Name) {
			return School{}, ErrConflict
		}
	}
	now := time.Now().UTC()
	item := School{ID: s.newID(), OrganizationID: orgID, Name: params.Name, Address: params.Address, ContactPhone: params.ContactPhone, Status: "active", CreatedAt: now, UpdatedAt: now}
	s.schools = append(s.schools, item)
	return item, nil
}

func (s *MemoryStore) ListAcademicTerms(context.Context, uint64) ([]AcademicTerm, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return slices.Clone(s.terms), nil
}

func (s *MemoryStore) CreateAcademicTerm(_ context.Context, orgID uint64, params CreateAcademicTermParams) (AcademicTerm, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, item := range s.terms {
		if item.OrganizationID == orgID && strings.EqualFold(item.Name, params.Name) {
			return AcademicTerm{}, ErrConflict
		}
	}
	now := time.Now().UTC()
	item := AcademicTerm{ID: s.newID(), OrganizationID: orgID, Name: params.Name, StartsOn: params.StartsOn, EndsOn: params.EndsOn, IsCurrent: params.IsCurrent, Status: "active", CreatedAt: now, UpdatedAt: now}
	if item.IsCurrent {
		for index := range s.terms {
			s.terms[index].IsCurrent = false
		}
	}
	s.terms = append(s.terms, item)
	return item, nil
}

func (s *MemoryStore) ListSchoolClasses(context.Context, uint64) ([]SchoolClass, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return slices.Clone(s.schoolClasses), nil
}

func (s *MemoryStore) CreateSchoolClass(_ context.Context, orgID uint64, params CreateSchoolClassParams) (SchoolClass, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, item := range s.schoolClasses {
		if item.OrganizationID == orgID && item.SchoolID == params.SchoolID && item.TermID == params.TermID && strings.EqualFold(item.Grade, params.Grade) && strings.EqualFold(item.Name, params.Name) {
			return SchoolClass{}, ErrConflict
		}
	}
	now := time.Now().UTC()
	item := SchoolClass{ID: s.newID(), OrganizationID: orgID, SchoolID: params.SchoolID, TermID: params.TermID, Grade: params.Grade, Name: params.Name, Status: "active", CreatedAt: now, UpdatedAt: now}
	s.schoolClasses = append(s.schoolClasses, item)
	return item, nil
}

func (s *MemoryStore) ListCareClasses(context.Context, uint64) ([]CareClass, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return slices.Clone(s.careClasses), nil
}

func (s *MemoryStore) CreateCareClass(_ context.Context, orgID uint64, params CreateCareClassParams) (CareClass, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, item := range s.careClasses {
		if item.OrganizationID == orgID && strings.EqualFold(item.Name, params.Name) {
			return CareClass{}, ErrConflict
		}
	}
	now := time.Now().UTC()
	item := CareClass{ID: s.newID(), OrganizationID: orgID, Name: params.Name, Capacity: params.Capacity, Status: "active", CreatedAt: now, UpdatedAt: now}
	s.careClasses = append(s.careClasses, item)
	return item, nil
}

func (s *MemoryStore) ListStudents(context.Context, uint64) ([]Student, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return slices.Clone(s.students), nil
}

func (s *MemoryStore) CreateStudent(_ context.Context, orgID uint64, params CreateStudentParams) (Student, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now().UTC()
	item := Student{ID: s.newID(), OrganizationID: orgID, SchoolID: params.SchoolID, TermID: params.TermID, SchoolClassID: params.SchoolClassID, CareClassID: cloneID(params.CareClassID), Name: params.Name, Gender: params.Gender, BirthDate: cloneTime(params.BirthDate), StudentNo: params.StudentNo, GuardianPhone: params.GuardianPhone, EmergencyContact: params.EmergencyContact, EmergencyPhone: params.EmergencyPhone, Status: "active", Notes: params.Notes, CreatedAt: now, UpdatedAt: now}
	s.students = append(s.students, item)
	return item, nil
}

func (s *MemoryStore) BulkCreateStudents(_ context.Context, orgID uint64, params BulkCreateStudentsParams) ([]Student, error) {
	if len(params.Items) == 0 {
		return []Student{}, nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now().UTC()
	created := make([]Student, 0, len(params.Items))
	for _, itemParams := range params.Items {
		item := Student{
			ID: s.newID(), OrganizationID: orgID, SchoolID: itemParams.SchoolID,
			TermID: itemParams.TermID, SchoolClassID: itemParams.SchoolClassID,
			CareClassID: cloneID(itemParams.CareClassID), Name: itemParams.Name,
			Gender: itemParams.Gender, BirthDate: cloneTime(itemParams.BirthDate),
			StudentNo: itemParams.StudentNo, GuardianPhone: itemParams.GuardianPhone,
			EmergencyContact: itemParams.EmergencyContact, EmergencyPhone: itemParams.EmergencyPhone,
			Status: "active", Notes: itemParams.Notes, CreatedAt: now, UpdatedAt: now,
		}
		created = append(created, item)
	}
	s.students = append(s.students, created...)
	return slices.Clone(created), nil
}

func (s *MemoryStore) FindStudent(_ context.Context, orgID, id uint64) (Student, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, item := range s.students {
		if item.OrganizationID == orgID && item.ID == id {
			return item, nil
		}
	}
	return Student{}, fmt.Errorf("%w: student %d", ErrNotFound, id)
}

func (s *MemoryStore) UpdateStudent(_ context.Context, orgID uint64, params UpdateStudentParams) (Student, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for index := range s.students {
		if s.students[index].OrganizationID != orgID || s.students[index].ID != params.ID {
			continue
		}
		item := &s.students[index]
		item.SchoolID, item.TermID, item.SchoolClassID = params.SchoolID, params.TermID, params.SchoolClassID
		item.CareClassID, item.Name, item.Gender = cloneID(params.CareClassID), params.Name, params.Gender
		item.BirthDate, item.StudentNo = cloneTime(params.BirthDate), params.StudentNo
		item.GuardianPhone, item.EmergencyContact, item.EmergencyPhone = params.GuardianPhone, params.EmergencyContact, params.EmergencyPhone
		item.Status, item.Notes, item.UpdatedAt = params.Status, params.Notes, time.Now().UTC()
		return *item, nil
	}
	return Student{}, fmt.Errorf("%w: student %d", ErrNotFound, params.ID)
}

func cloneID(value *uint64) *uint64 {
	if value == nil {
		return nil
	}
	copy := *value
	return &copy
}

func cloneTime(value *time.Time) *time.Time {
	if value == nil {
		return nil
	}
	copy := *value
	return &copy
}
