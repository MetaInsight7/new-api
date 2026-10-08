import { z } from 'zod'

export const governanceReviewNoteSchema = z.object({ note: z.string() })

export type GovernanceReviewNoteFormValues = z.infer<
  typeof governanceReviewNoteSchema
>
