import { Button as ButtonPrimitive } from '@base-ui/react/button';
import type { VariantProps } from 'class-variance-authority';
import { cva } from 'class-variance-authority';

import { cn } from '@/lib';

const buttonVariants = cva(
  "inline-flex shrink-0 cursor-pointer touch-manipulation items-center justify-center gap-2 rounded-control border border-transparent text-base leading-tight font-semibold whitespace-nowrap select-none transition-control disabled:pointer-events-none disabled:opacity-50 motion-safe:active:translate-y-px [&_svg]:pointer-events-none [&_svg]:shrink-0 [&_svg:not([class*='size-'])]:size-5",
  {
    variants: {
      variant: {
        default:
          'border-primary-border bg-primary text-primary-foreground hover:bg-primary-hover active:bg-primary-press',
        outline: 'border-input bg-transparent text-foreground hover:bg-muted',
        link: 'border-0 font-normal text-foreground underline decoration-1 underline-offset-4 hover:decoration-2 motion-safe:active:translate-y-0',
      },
      size: {
        default: 'h-11 px-4',
        lg: 'h-12 px-5',
      },
    },
    compoundVariants: [{ variant: 'link', class: 'px-0' }],
    defaultVariants: {
      variant: 'default',
      size: 'default',
    },
  }
);

function Button({
  className,
  variant = 'default',
  size = 'default',
  ...props
}: ButtonPrimitive.Props & VariantProps<typeof buttonVariants>) {
  return (
    <ButtonPrimitive
      data-slot="button"
      className={cn(buttonVariants({ variant, size, className }))}
      {...props}
    />
  );
}

export { Button, buttonVariants };
