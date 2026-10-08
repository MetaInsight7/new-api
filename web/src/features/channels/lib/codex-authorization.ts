import { z } from 'zod'

export const codexAuthorizationSchema = z.object({
  input: z.string().trim().min(1, 'Required'),
})

export type CodexAuthorizationFormValues = z.infer<
  typeof codexAuthorizationSchema
>
