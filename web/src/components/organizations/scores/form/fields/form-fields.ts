import type { ScoreFormValues } from '../types';

export interface FormFieldsProps {
  form: ScoreFormValues;
  set: <K extends keyof ScoreFormValues>(key: K, value: ScoreFormValues[K]) => void;
}

export function textField(
  { form, set }: FormFieldsProps,
  key: 'start' | 'end' | 'search' | 'repository' | 'userId' | 'kinds'
) {
  return {
    value: form[key],
    onChange: (event: { target: { value: string } }) => set(key, event.target.value),
  };
}
