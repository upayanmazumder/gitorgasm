import { cva } from 'class-variance-authority';

export const panelVariants = cva('rounded-panel border', {
  variants: {
    tone: {
      raised: 'border-border bg-card shadow-panel',
      dashed: 'border-dashed border-input',
    },
    padding: {
      default: 'p-4 md:p-6',
      none: 'overflow-hidden',
    },
  },
  defaultVariants: {
    tone: 'raised',
    padding: 'default',
  },
});
