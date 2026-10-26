import * as z from 'zod';

export const emailSchema = z.email();
export const urlSchema = z.url();
export const uuidSchema = z.uuid();
export const nonEmptyStringSchema = z.string().min(1);
export const positiveIntSchema = z.number().int().positive();
export const dateTimeSchema = z.iso.datetime({ offset: true });

export const cursorQuerySchema = z.object({
  limit: z.number().int().min(1).max(100).optional(),
  cursor: z.string().min(1).optional(),
});

export function cursorPageSchema<T extends z.ZodType>(itemSchema: T) {
  return z.object({
    data: z.array(itemSchema),
    next_cursor: z.string().nullable(),
  });
}
