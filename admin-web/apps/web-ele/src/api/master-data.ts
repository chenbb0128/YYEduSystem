import { businessRequestClient } from '#/api/request';

export interface MasterSummary {
  academic_terms: number;
  care_classes: number;
  school_classes: number;
  schools: number;
  students: number;
}

export interface SchoolRecord {
  id: number;
  name: string;
  address: string;
  contact_phone: string;
  status: string;
}

export interface AcademicTermRecord {
  id: number;
  name: string;
  starts_on: string;
  ends_on: string;
  is_current: boolean;
  status: string;
}

export interface SchoolClassRecord {
  id: number;
  school_id: number;
  term_id: number;
  grade: string;
  name: string;
  status: string;
}

export interface CareClassRecord {
  id: number;
  name: string;
  capacity: number;
  status: string;
}

export interface StudentRecord {
  id: number;
  school_id: number;
  term_id: number;
  school_class_id: number;
  care_class_id?: number;
  name: string;
  gender: 'female' | 'male' | 'unknown';
  birth_date?: string;
  student_no: string;
  guardian_phone: string;
  emergency_contact: string;
  emergency_phone: string;
  status: 'active' | 'inactive';
  notes: string;
}

export interface StudentPayload {
  school_id: number;
  term_id: number;
  school_class_id: number;
  care_class_id?: number;
  name: string;
  gender: StudentRecord['gender'];
  birth_date?: string;
  student_no: string;
  guardian_phone: string;
  emergency_contact: string;
  emergency_phone: string;
  status?: StudentRecord['status'];
  notes: string;
}

export interface StudentProfilePayload {
  school_name: string;
  term_id?: number;
  term_name?: string;
  grade: string;
  class_name: string;
  care_class_name?: string;
  name: string;
  gender: StudentRecord['gender'];
  birth_date?: string;
  student_no: string;
  guardian_phone: string;
  emergency_contact: string;
  emergency_phone: string;
  status?: StudentRecord['status'];
  notes: string;
}

export interface StudentImportIssue {
  row: number;
  name?: string;
  field?: string;
  reason: string;
}

export interface StudentImportResult {
  created: StudentRecord[];
  skipped_duplicates: StudentImportIssue[];
  invalid: StudentImportIssue[];
}

export interface PageResult<T> {
  items: T[];
  total: number;
}

export function getMasterSummaryApi() {
  return businessRequestClient.get<MasterSummary>('/summary');
}

export function getSchoolsApi() {
  return businessRequestClient.get<PageResult<SchoolRecord>>('/schools');
}

export function createSchoolApi(
  data: Pick<SchoolRecord, 'address' | 'contact_phone' | 'name'>,
) {
  return businessRequestClient.post<SchoolRecord>('/schools', data);
}

export function getAcademicTermsApi() {
  return businessRequestClient.get<PageResult<AcademicTermRecord>>(
    '/academic-terms',
  );
}

export function createAcademicTermApi(
  data: Pick<
    AcademicTermRecord,
    'ends_on' | 'is_current' | 'name' | 'starts_on'
  >,
) {
  return businessRequestClient.post<AcademicTermRecord>(
    '/academic-terms',
    data,
  );
}

export function getSchoolClassesApi() {
  return businessRequestClient.get<PageResult<SchoolClassRecord>>(
    '/school-classes',
  );
}

export function createSchoolClassApi(
  data: Pick<SchoolClassRecord, 'grade' | 'name' | 'school_id' | 'term_id'>,
) {
  return businessRequestClient.post<SchoolClassRecord>('/school-classes', data);
}

export function getCareClassesApi() {
  return businessRequestClient.get<PageResult<CareClassRecord>>(
    '/care-classes',
  );
}

export function createCareClassApi(
  data: Pick<CareClassRecord, 'capacity' | 'name'>,
) {
  return businessRequestClient.post<CareClassRecord>('/care-classes', data);
}

export function getStudentsApi(params?: {
  care_class_id?: number;
  grade?: string;
  keyword?: string;
  school_class_id?: number;
  school_id?: number;
  status?: StudentRecord['status'];
  term_id?: number;
}) {
  return businessRequestClient.get<PageResult<StudentRecord>>('/students', {
    params,
  });
}

export function createStudentProfileApi(data: StudentProfilePayload) {
  return businessRequestClient.post<StudentRecord>('/students/profile', data);
}

export function importStudentsApi(data: { items: StudentProfilePayload[] }) {
  return businessRequestClient.post<StudentImportResult>(
    '/students/import',
    data,
  );
}

export function updateStudentProfileApi(
  id: number,
  data: StudentProfilePayload,
) {
  return businessRequestClient.put<StudentRecord>(
    `/students/${id}/profile`,
    data,
  );
}

export function createStudentApi(data: StudentPayload) {
  return businessRequestClient.post<StudentRecord>('/students', data);
}

export function updateStudentApi(id: number, data: StudentPayload) {
  return businessRequestClient.put<StudentRecord>(`/students/${id}`, data);
}
