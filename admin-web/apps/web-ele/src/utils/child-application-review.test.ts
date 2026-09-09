import type { ReviewChildApplicationPayload } from '#/api/child-applications';

import { describe, expect, it } from 'vitest';

import {
  extractReviewErrorMessage,
  submitChildApplicationReview,
} from './child-application-review';

describe('child application review flow', () => {
  it('submits the review directly without a preflight list refresh', async () => {
    const calls: Array<{
      id: number;
      payload: ReviewChildApplicationPayload;
    }> = [];

    await submitChildApplicationReview({
      applicationId: 12,
      createSchoolClass: true,
      reviewNote: '已核对',
      review: async (id, payload) => {
        calls.push({ id, payload });
        return { status: 'approved' };
      },
      schoolClassId: undefined,
      status: 'approved',
      studentId: undefined,
    });

    expect(calls).toEqual([
      {
        id: 12,
        payload: {
          create_school_class: true,
          review_note: '已核对',
          status: 'approved',
        },
      },
    ]);
  });

  it('prefers the backend message when a review request fails', () => {
    const error = {
      response: {
        data: { message: '请先选择孩子所在的学校班级' },
        status: 400,
      },
    };

    expect(extractReviewErrorMessage(error)).toBe('请先选择孩子所在的学校班级');
  });
});
