import { act, render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

import { WavingMascot } from '../waving-mascot';

const BUTTON = { name: 'Make the Gitorgasm cat wave' };
const GREETING = { name: 'The Gitorgasm cat waving hello' };

let preferenceListeners = new Set<() => void>();
let reducedMotion = false;

function setReducedMotion(value: boolean) {
  reducedMotion = value;
  act(() => preferenceListeners.forEach(listener => listener()));
}

beforeEach(() => {
  preferenceListeners = new Set();
  reducedMotion = false;
  vi.stubGlobal('matchMedia', (query: string) => ({
    matches: reducedMotion && query.includes('reduce'),
    addEventListener: (_type: string, listener: () => void) => preferenceListeners.add(listener),
    removeEventListener: (_type: string, listener: () => void) =>
      preferenceListeners.delete(listener),
  }));
  // jsdom has no IntersectionObserver; the visibility trigger is not under test here.
  vi.stubGlobal(
    'IntersectionObserver',
    class {
      observe() {}
      disconnect() {}
    }
  );
});

afterEach(() => vi.unstubAllGlobals());

describe('WavingMascot', () => {
  it('is a named button that keyboard users can reach and press repeatedly', async () => {
    const user = userEvent.setup();
    render(<WavingMascot />);

    await user.tab();
    expect(screen.getByRole('button', BUTTON)).toHaveFocus();

    await user.keyboard('{Enter}');
    expect(screen.getByRole('button', BUTTON)).toHaveFocus();
    await user.keyboard(' ');
    expect(screen.getByRole('button', BUTTON)).toHaveFocus();
    await user.click(screen.getByRole('button', BUTTON));

    expect(screen.getByRole('button', BUTTON)).toHaveFocus();
  });

  it('shows a static greeting without a control when motion is reduced', () => {
    reducedMotion = true;
    render(<WavingMascot />);

    expect(screen.getByRole('img', GREETING)).toBeInTheDocument();
    expect(screen.queryByRole('button')).not.toBeInTheDocument();
  });

  it('follows a change of the motion preference', () => {
    render(<WavingMascot />);
    expect(screen.getByRole('button', BUTTON)).toBeInTheDocument();

    setReducedMotion(true);

    expect(screen.queryByRole('button')).not.toBeInTheDocument();
    expect(screen.getByRole('img', GREETING)).toBeInTheDocument();
  });
});
