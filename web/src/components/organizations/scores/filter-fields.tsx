import { inputClassName as input, textField, type FormFieldsProps } from './form-fields';

export function FilterFields({ form, set }: FormFieldsProps) {
  const text = (key: 'search' | 'repository' | 'userId' | 'kinds') => textField({ form, set }, key);

  return (
    <>
      <label className="grid gap-1">
        Search people or repositories
        <input {...text('search')} />
      </label>
      <label className="grid gap-1">
        Repository (full name)
        <input {...text('repository')} />
      </label>
      <label className="grid gap-1">
        GitHub user ID
        <input type="number" min="1" {...text('userId')} />
      </label>
      <label className="grid gap-1">
        Activity types (comma separated)
        <input placeholder="commit, review" {...text('kinds')} />
      </label>
      <label className="flex items-center gap-2">
        <input
          type="checkbox"
          checked={form.excludeBots}
          onChange={e => set('excludeBots', e.target.checked)}
        />
        Hide bots
      </label>
      <details className="sm:col-span-2">
        <summary>Custom scoring rules</summary>
        <label className="grid gap-1">
          Complete scoring rules in JSON
          <textarea
            value={form.rules}
            onChange={e => set('rules', e.target.value)}
            rows={6}
            className={`${input} font-mono text-sm`}
          />
        </label>
      </details>
    </>
  );
}
