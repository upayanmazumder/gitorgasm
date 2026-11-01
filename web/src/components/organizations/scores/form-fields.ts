import type { ScoreForm } from './types';

export interface FormFieldsProps {
  form: ScoreForm;
  set: <K extends keyof ScoreForm>(key: K, value: ScoreForm[K]) => void;
}

export const inputClassName = 'rounded border p-2';

export function textField(
  { form, set }: FormFieldsProps,
  key: 'start' | 'end' | 'search' | 'repository' | 'userId' | 'kinds'
) {
  return {
    value: form[key],
    onChange: (event: { target: { value: string } }) => set(key, event.target.value),
    className: inputClassName,
  };
}
