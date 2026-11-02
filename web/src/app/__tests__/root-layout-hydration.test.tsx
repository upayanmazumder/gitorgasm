import { act, useState, type ReactNode } from 'react';
import { within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { hydrateRoot, type Root } from 'react-dom/client';
import { renderToString } from 'react-dom/server';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

import { Button } from '@/components/ui';

import RootLayout from '../layout';

// next/font needs the Next.js compiler.
vi.mock('@/styles', () => ({ fontVariables: '' }));
// The adapter needs the App Router context.
vi.mock('nuqs/adapters/next/app', () => ({
  NuqsAdapter: ({ children }: { children: ReactNode }) => children,
}));

let side = 'server';

function Probe() {
  const [count, setCount] = useState(0);
  return (
    <>
      <p>{side}</p>
      <Button onClick={() => setCount(value => value + 1)}>Count {count}</Button>
    </>
  );
}

const originalRoot = document.documentElement;
let root: Root | undefined;

function installServerDocument() {
  const html = renderToString(
    <RootLayout>
      <Probe />
    </RootLayout>
  );
  const parsed = new DOMParser().parseFromString(html, 'text/html').documentElement;
  document.replaceChild(document.importNode(parsed, true), document.documentElement);
}

async function hydrate() {
  const spy = vi.spyOn(console, 'error').mockImplementation(() => {});
  const onRecoverableError = vi.fn();
  await act(async () => {
    root = hydrateRoot(
      document,
      <RootLayout>
        <Probe />
      </RootLayout>,
      { onRecoverableError }
    );
  });
  const logged = (pattern: RegExp) =>
    spy.mock.calls.map(args => args.map(String).join(' ')).filter(line => pattern.test(line));
  return { onRecoverableError, logged };
}

beforeEach(() => {
  vi.stubGlobal('IS_REACT_ACT_ENVIRONMENT', true);
  // jsdom has no matchMedia. next-themes and sonner subscribe to it in effects.
  vi.stubGlobal('matchMedia', () => ({
    matches: false,
    addListener: vi.fn(),
    removeListener: vi.fn(),
    addEventListener: vi.fn(),
    removeEventListener: vi.fn(),
  }));
});

afterEach(async () => {
  await act(async () => root?.unmount());
  root = undefined;
  document.replaceChild(originalRoot, document.documentElement);
  side = 'server';
  vi.restoreAllMocks();
  vi.unstubAllGlobals();
});

describe('RootLayout hydration', () => {
  it('ignores an extension attribute on body', async () => {
    installServerDocument();
    document.body.setAttribute('cz-shortcut-listen', 'true');

    const { onRecoverableError, logged } = await hydrate();

    expect(logged(/hydrat|didn't match/i)).toEqual([]);
    expect(onRecoverableError).not.toHaveBeenCalled();
    expect(document.body).toHaveAttribute('cz-shortcut-listen', 'true');
    await userEvent.setup().click(within(document.body).getByRole('button', { name: 'Count 0' }));
    expect(within(document.body).getByRole('button', { name: 'Count 1' })).toBeInTheDocument();
  });

  it('still reports a mismatch inside body', async () => {
    installServerDocument();
    side = 'client';

    const { onRecoverableError, logged } = await hydrate();

    expect([...onRecoverableError.mock.calls, ...logged(/hydrat/i)]).not.toHaveLength(0);
    expect(within(document.body).getByText('client')).toBeInTheDocument();
    await userEvent.setup().click(within(document.body).getByRole('button', { name: 'Count 0' }));
    expect(within(document.body).getByRole('button', { name: 'Count 1' })).toBeInTheDocument();
  });
});
