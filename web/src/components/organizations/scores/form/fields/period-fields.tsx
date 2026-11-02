import { Field, Input, NativeSelect } from '@/components/ui';

import { textField, type FormFieldsProps } from './form-fields';

export function PeriodFields({ form, set }: FormFieldsProps) {
  const text = (key: 'start' | 'end') => textField({ form, set }, key);

  return (
    <>
      <Field label="Time period (UTC)">
        <NativeSelect value={form.period} onChange={e => set('period', e.target.value)}>
          <option value="last30_days">Last 30 days</option>
          <option value="last7_days">Last 7 days</option>
          <option value="today">Today</option>
          <option value="this_week">This week</option>
          <option value="this_month">This month</option>
          <option value="all">All available time</option>
          <option value="custom">Choose dates and times</option>
        </NativeSelect>
      </Field>
      <Field label="Activity">
        <NativeSelect value={form.category} onChange={e => set('category', e.target.value)}>
          <option value="all">All</option>
          <option value="code">Code</option>
          <option value="reviews">Reviews</option>
          <option value="issues">Issues</option>
        </NativeSelect>
      </Field>
      {form.period === 'custom' && (
        <>
          <Field label="Start date and time">
            <Input type="datetime-local" required {...text('start')} />
          </Field>
          <Field label="End date and time">
            <Input type="datetime-local" required {...text('end')} />
          </Field>
        </>
      )}
    </>
  );
}
