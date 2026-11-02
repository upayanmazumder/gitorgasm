import { act, renderHook } from '@testing-library/react';
import { describe, expect, it, vi } from 'vitest';
import * as z from 'zod';

import { useZodForm } from '..';

const schema = z.object({
  name: z.string().trim().min(1, 'Name is required'),
  age: z.coerce.number().int().positive(),
});

describe('useZodForm', () => {
  it('reports schema errors per field and does not submit', async () => {
    const onValid = vi.fn();
    const onInvalid = vi.fn();
    const { result } = renderHook(() =>
      useZodForm(schema, { defaultValues: { name: '', age: 30 } })
    );

    await act(() => result.current.handleSubmit(onValid, onInvalid)());

    expect(onValid).not.toHaveBeenCalled();
    expect(onInvalid).toHaveBeenCalledOnce();
    expect(onInvalid.mock.calls[0][0].name?.message).toBe('Name is required');
    expect(onInvalid.mock.calls[0][0].age).toBeUndefined();
  });
  it('submits the parsed output of the schema', async () => {
    const onValid = vi.fn();
    const { result } = renderHook(() =>
      useZodForm(schema, { defaultValues: { name: '  Alice  ', age: '42' } })
    );

    await act(() => result.current.handleSubmit(onValid)());

    expect(onValid).toHaveBeenCalledWith({ name: 'Alice', age: 42 }, undefined);
  });
});
