import type { ComponentProps } from 'react';

interface Props extends Omit<ComponentProps<'input'>, 'type'> {
  label: string;
}

export function CheckboxField({ label, ...props }: Props) {
  return (
    <label className="flex min-h-11 cursor-pointer items-center gap-3">
      <input type="checkbox" className="size-5" {...props} />
      {label}
    </label>
  );
}
