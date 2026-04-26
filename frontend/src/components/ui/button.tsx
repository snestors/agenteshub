import * as React from "react";
import { cva, type VariantProps } from "class-variance-authority";
import { cn } from "@/lib/utils";

// HUD-styled button. clip-tag corner cut, accent border + glow on hover.
const buttonVariants = cva(
  "inline-flex items-center justify-center gap-2 whitespace-nowrap font-mono uppercase tracking-hud-tight text-[11px] font-semibold transition-all disabled:pointer-events-none disabled:opacity-40 cursor-pointer select-none",
  {
    variants: {
      variant: {
        primary:
          "clip-tag bg-[rgba(94,240,255,0.10)] border border-[rgba(94,240,255,0.55)] text-[var(--color-cyan)] hover:bg-[rgba(94,240,255,0.18)] hover:glow-cyan",
        magenta:
          "clip-tag bg-[rgba(255,78,214,0.10)] border border-[rgba(255,78,214,0.55)] text-[var(--color-magenta)] hover:bg-[rgba(255,78,214,0.18)] hover:glow-magenta",
        ghost:
          "border border-transparent text-[var(--color-dim)] hover:text-[var(--color-fg)] hover:border-[rgba(120,255,220,0.18)]",
        danger:
          "clip-tag bg-[rgba(255,92,122,0.10)] border border-[rgba(255,92,122,0.55)] text-[var(--color-danger)] hover:bg-[rgba(255,92,122,0.18)]",
      },
      size: {
        default: "h-9 px-4",
        sm: "h-7 px-3 text-[10px]",
        lg: "h-11 px-6 text-[12px]",
        icon: "h-9 w-9",
      },
    },
    defaultVariants: {
      variant: "primary",
      size: "default",
    },
  }
);

export interface ButtonProps
  extends React.ButtonHTMLAttributes<HTMLButtonElement>,
    VariantProps<typeof buttonVariants> {}

const Button = React.forwardRef<HTMLButtonElement, ButtonProps>(
  ({ className, variant, size, ...props }, ref) => (
    <button
      ref={ref}
      className={cn(buttonVariants({ variant, size }), className)}
      {...props}
    />
  )
);
Button.displayName = "Button";

export { Button, buttonVariants };
