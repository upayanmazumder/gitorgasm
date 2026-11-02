import { CheckboxField, Field, Input, Textarea } from '@/components/ui';

import { textField, type FormFieldsProps } from './form-fields';

export function FilterFields({ form, set }: FormFieldsProps) {
  const text = (key: 'search' | 'repository' | 'userId' | 'kinds') => textField({ form, set }, key);

  return (
    <>
      <Field label="Search people or repositories">
        <Input {...text('search')} />
      </Field>
      <Field label="Repository (full name)">
        <Input {...text('repository')} />
      </Field>
      <Field label="GitHub user ID">
        <Input type="number" min="1" {...text('userId')} />
      </Field>
      <Field label="Activity types (comma separated)">
        <Input placeholder="commit, review" {...text('kinds')} />
      </Field>
      <CheckboxField
        label="Hide bots"
        checked={form.excludeBots}
        onChange={e => set('excludeBots', e.target.checked)}
      />
      <details className="md:col-span-2">
        <summary>Custom scoring rules</summary>
        <Field label="Complete scoring rules in JSON" className="mt-2">
          <Textarea
            value={form.rules}
            onChange={e => set('rules', e.target.value)}
            rows={6}
            className="font-mono text-sm"
          />
        </Field>
      </details>
    </>
  );
}
