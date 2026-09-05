import { businessRequestClient } from '#/api/request';

export type WrongQuestionStatus = 'active' | 'archived' | 'mastered';

export interface WrongQuestionRecord {
  id: number;
  student_id: number;
  student_name: string;
  subject: string;
  question_text: string;
  answer_text: string;
  explanation: string;
  knowledge_point: string;
  source_image_url: string;
  source_homework_task_id?: number;
  teacher_note: string;
  status: WrongQuestionStatus;
  created_by_user_id?: number;
  created_by_name: string;
  last_reviewed_at?: string;
  created_at: string;
  updated_at: string;
}

export interface ExtractedWrongQuestionRecord {
  temp_id: string;
  subject: string;
  question_text: string;
  answer_text: string;
  explanation: string;
  knowledge_point: string;
  confidence: number;
}

export interface WrongPaperRecord {
  id: number;
  student_id: number;
  student_name: string;
  title: string;
  source: 'parent' | 'system' | 'teacher';
  status: 'archived' | 'assigned' | 'generated';
  generated_by_type: 'parent' | 'staff' | 'system';
  generated_by_user_id?: number;
  question_count: number;
  questions?: WrongQuestionRecord[];
  created_at: string;
  updated_at: string;
}

export interface PageResult<T> {
  items: T[];
  total: number;
}

export function extractWrongQuestionsApi(data: {
  image_url?: string;
  source_text?: string;
  subject?: string;
}) {
  return businessRequestClient.post<{
    image_url: string;
    items: ExtractedWrongQuestionRecord[];
    mocked: boolean;
    total: number;
  }>('/wrong-questions/extract', data);
}

export function getWrongQuestionsApi(params?: {
  keyword?: string;
  status?: '' | WrongQuestionStatus;
  student_id?: number;
  subject?: string;
}) {
  return businessRequestClient.get<PageResult<WrongQuestionRecord>>(
    '/wrong-questions',
    { params },
  );
}

export function createWrongQuestionsApi(data: {
  items: Array<{
    answer_text?: string;
    explanation?: string;
    knowledge_point?: string;
    question_text: string;
    source_homework_task_id?: number;
    source_image_url?: string;
    student_id: number;
    subject: string;
    teacher_note?: string;
  }>;
}) {
  return businessRequestClient.post<PageResult<WrongQuestionRecord>>(
    '/wrong-questions/bulk',
    data,
  );
}

export function updateWrongQuestionApi(
  id: number,
  data: {
    answer_text?: string;
    explanation?: string;
    knowledge_point?: string;
    question_text: string;
    status: WrongQuestionStatus;
    subject: string;
    teacher_note?: string;
  },
) {
  return businessRequestClient.put<WrongQuestionRecord>(
    `/wrong-questions/${id}`,
    data,
  );
}

export function getWrongPapersApi(params?: { student_id?: number }) {
  return businessRequestClient.get<PageResult<WrongPaperRecord>>(
    '/wrong-papers',
    { params },
  );
}

export function getWrongPaperApi(id: number) {
  return businessRequestClient.get<WrongPaperRecord>(`/wrong-papers/${id}`);
}

export function createWrongPaperApi(data: {
  question_ids: number[];
  student_id: number;
  title?: string;
}) {
  return businessRequestClient.post<WrongPaperRecord>('/wrong-papers', data);
}
