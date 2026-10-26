import { zodResolver } from '@hookform/resolvers/zod';
import {
  useForm,
  type FieldValues,
  type Resolver,
  type UseFormProps,
  type UseFormReturn,
} from 'react-hook-form';
import type * as z from 'zod';

// Preserve the schema's input and parsed output types.
export function useZodForm<TSchema extends z.ZodType<FieldValues, FieldValues>>(
  schema: TSchema,
  options?: Omit<UseFormProps<z.input<TSchema>, unknown, z.output<TSchema>>, 'resolver'>
): UseFormReturn<z.input<TSchema>, unknown, z.output<TSchema>> {
  return useForm<z.input<TSchema>, unknown, z.output<TSchema>>({
    ...options,
    // The resolver overload loses the concrete schema type inside this generic helper.
    resolver: zodResolver(schema) as unknown as Resolver<
      z.input<TSchema>,
      unknown,
      z.output<TSchema>
    >,
  });
}
