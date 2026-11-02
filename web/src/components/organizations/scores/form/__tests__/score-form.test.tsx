import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { describe, expect, it, vi } from 'vitest';

import { defaultForm } from '../form-query';
import { ScoreForm } from '../score-form';

function setup(props: { disabled?: boolean; pending?: boolean } = {}) {
  const onSubmit = vi.fn();
  render(
    <ScoreForm
      form={defaultForm}
      onChange={vi.fn()}
      onSubmit={onSubmit}
      disabled={props.disabled ?? false}
      pending={props.pending ?? false}
    />
  );
  return onSubmit;
}

describe('ScoreForm', () => {
  it('calls onSubmit once when Show scores is pressed', async () => {
    const onSubmit = setup();

    await userEvent.setup().click(screen.getByRole('button', { name: 'Show scores' }));

    expect(onSubmit).toHaveBeenCalledTimes(1);
  });

  it('stays disabled and does not submit while pending', async () => {
    const onSubmit = setup({ pending: true });
    const button = screen.getByRole('button', { name: 'Calculating…' });

    expect(button).toBeDisabled();
    await userEvent.setup().click(button);

    expect(onSubmit).not.toHaveBeenCalled();
  });
});
