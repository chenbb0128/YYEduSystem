package masterdata

import (
	"context"
	"errors"
)

var (
	ErrNotFound = errors.New("masterdata: resource not found")
	ErrConflict = errors.New("masterdata: resource already exists")
)

type Store interface {
	ListSchools(context.Context, uint64) ([]School, error)
	CreateSchool(context.Context, uint64, CreateSchoolParams) (School, error)
	ListAcademicTerms(context.Context, uint64) ([]AcademicTerm, error)
	CreateAcademicTerm(context.Context, uint64, CreateAcademicTermParams) (AcademicTerm, error)
	ListSchoolClasses(context.Context, uint64) ([]SchoolClass, error)
	CreateSchoolClass(context.Context, uint64, CreateSchoolClassParams) (SchoolClass, error)
	ListCareClasses(context.Context, uint64) ([]CareClass, error)
	CreateCareClass(context.Context, uint64, CreateCareClassParams) (CareClass, error)
	ListStudents(context.Context, uint64) ([]Student, error)
	CreateStudent(context.Context, uint64, CreateStudentParams) (Student, error)
	BulkCreateStudents(context.Context, uint64, BulkCreateStudentsParams) ([]Student, error)
	FindStudent(context.Context, uint64, uint64) (Student, error)
	UpdateStudent(context.Context, uint64, UpdateStudentParams) (Student, error)
}
