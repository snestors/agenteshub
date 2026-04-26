import * as React from "react";
import { cn } from "@/lib/utils";

export type InputProps = React.InputHTMLAttributes<HTMLInputElement>;

const Input = React.forwardRef<HTMLInputElement, InputProps>(
  ({ className, type = "text", ...props }, ref) => (
    <input
      ref={ref}
      type={type}
      className={cn(
        // HUD: dark inset, cyan focus border, mono font
        "w-full bg-[rgba(0,0,0,0.35)] border border-[rgba(120,255,220,0.18)]",
        "px-3 py-2 text-[13px] font-mono text-[var(--color-fg)] placeholder:text-[var(--color-dim)]",
        "transition-colors clip-hud-sm",
        "focus:border-[var(--color-cyan)] focus:bg-[rgba(0,0,0,0.55)]",
        "disabled:opacity-50",
        className
      )}
      {...props}
    />
  )
);
Input.displayName = "Input";

export { Input };
