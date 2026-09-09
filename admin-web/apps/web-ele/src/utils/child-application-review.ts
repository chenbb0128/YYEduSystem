import type {
  ChildApplicationStatus,
  ReviewChildApplicationPayload,
} from '#/api/child-applications';

export type ChildApplicationReviewStatus = Exclude<
  ChildApplicationStatus,
  'pending'
>;

interface SubmitChildApplicationReviewOptions<T> {
  applicationId: number;
  createSchoolClass: boolean;
  review: (
    applicationId: number,
    payload: ReviewChildApplicationPayload,
  ) => Promise<T>;
  reviewNote: string;
  schoolClassId?: number;
  status: ChildApplicationReviewStatus;
  studentId?: number;
}

function createReviewPayload(
  options: Omit<SubmitChildApplicationReviewOptions<unknown>, 'review'>,
): ReviewChildApplicationPayload {
  const payload: ReviewChildApplicationPayload = {
    review_note: options.reviewNote.trim() || undefined,
    status: options.status,
  };

  if (options.status === 'approved') {
    if (options.schoolClassId) {
      payload.school_class_id = options.schoolClassId;
    }
    if (options.studentId) {
      payload.student_id = options.studentId;
    }
    payload.create_school_class = options.createSchoolClass;
  }

  return payload;
}

export async function submitChildApplicationReview<T>(
  options: SubmitChildApplicationReviewOptions<T>,
) {
  return options.review(options.applicationId, createReviewPayload(options));
}

export function extractReviewErrorMessage(
  error: unknown,
  fallback = '家长入班申请审核失败',
) {
  if (error && typeof error === 'object') {
    const response = Reflect.get(error, 'response');
    const data =
      response && typeof response === 'object'
        ? Reflect.get(response, 'data')
        : undefined;
    if (data && typeof data === 'object') {
      const message =
        Reflect.get(data, 'message') ?? Reflect.get(data, 'error');
      if (typeof message === 'string' && message.trim()) {
        return message.trim();
      }
    }
  }

  if (error instanceof Error && error.message.trim()) {
    return error.message;
  }
  return fallback;
}
