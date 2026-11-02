import { useState } from 'react';

// Moves focus to an element that may not exist yet. The target waits in state until its ref callback runs.
export function usePendingFocus() {
  const [pending, setPending] = useState<string | null>(null);

  function register(target: string) {
    return (element: HTMLElement | null) => {
      if (element && pending === target) {
        element.focus();
        setPending(null);
      }
    };
  }

  return { focus: setPending, register };
}
