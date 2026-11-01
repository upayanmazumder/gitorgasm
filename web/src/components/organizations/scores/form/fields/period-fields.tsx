import { inputClassName as input, textField, type FormFieldsProps } from './form-fields';

export function PeriodFields({ form, set }: FormFieldsProps) {
  const text = (key: 'start' | 'end') => textField({ form, set }, key);

  return (
    <>
      <label className="grid gap-1">
        Time period (UTC)
        <select value={form.period} onChange={e => set('period', e.target.value)} className={input}>
          <option value="last30_days">Last 30 days</option>
          <option value="last7_days">Last 7 days</option>
          <option value="today">Today</option>
          <option value="this_week">This week</option>
          <option value="this_month">This month</option>
          <option value="all">All available time</option>
          <option value="custom">Choose dates and times</option>
        </select>
      </label>
      <label className="grid gap-1">
        Activity
        <select
          value={form.category}
          onChange={e => set('category', e.target.value)}
          className={input}
        >
          <option value="all">All</option>
          <option value="code">Code</option>
          <option value="reviews">Reviews</option>
          <option value="issues">Issues</option>
        </select>
      </label>
      {form.period === 'custom' && (
        <>
          <label className="grid gap-1">
            Start date and time
            <input type="datetime-local" required {...text('start')} />
          </label>
          <label className="grid gap-1">
            End date and time
            <input type="datetime-local" required {...text('end')} />
          </label>
        </>
      )}
    </>
  );
}
