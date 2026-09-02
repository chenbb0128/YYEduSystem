package masterdata

import "time"

const DefaultOrganizationID uint64 = 1

type School struct {
	ID             uint64
	OrganizationID uint64
	Name           string
	Address        string
	ContactPhone   string
	Status         string
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

type AcademicTerm struct {
	ID             uint64
	OrganizationID uint64
	Name           string
	StartsOn       time.Time
	EndsOn         time.Time
	IsCurrent      bool
	Status         string
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

type SchoolClass struct {
	ID             uint64
	OrganizationID uint64
	SchoolID       uint64
	TermID         uint64
	Grade          string
	Name           string
	Status         string
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

type CareClass struct {
	ID             uint64
	OrganizationID uint64
	Name           string
	Capacity       uint32
	Status         string
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

type Student struct {
	ID               uint64
	OrganizationID   uint64
	SchoolID         uint64
	TermID           uint64
	SchoolClassID    uint64
	CareClassID      *uint64
	Name             string
	Gender           string
	BirthDate        *time.Time
	StudentNo        string
	GuardianPhone    string
	EmergencyContact string
	EmergencyPhone   string
	Status           string
	Notes            string
	CreatedAt        time.Time
	UpdatedAt        time.Time
}

type CreateSchoolParams struct {
	Name         string
	Address      string
	ContactPhone string
}

type CreateAcademicTermParams struct {
	Name      string
	StartsOn  time.Time
	EndsOn    time.Time
	IsCurrent bool
}

type CreateSchoolClassParams struct {
	SchoolID uint64
	TermID   uint64
	Grade    string
	Name     string
}

type CreateCareClassParams struct {
	Name     string
	Capacity uint32
}

type CreateStudentParams struct {
	SchoolID         uint64
	TermID           uint64
	SchoolClassID    uint64
	CareClassID      *uint64
	Name             string
	Gender           string
	BirthDate        *time.Time
	StudentNo        string
	GuardianPhone    string
	EmergencyContact string
	EmergencyPhone   string
	Notes            string
}

type UpdateStudentParams struct {
	ID               uint64
	SchoolID         uint64
	TermID           uint64
	SchoolClassID    uint64
	CareClassID      *uint64
	Name             string
	Gender           string
	BirthDate        *time.Time
	StudentNo        string
	GuardianPhone    string
	EmergencyContact string
	EmergencyPhone   string
	Status           string
	Notes            string
}
