import * as React from "react";
import { cn } from "@/lib/utils";

export interface ProgressProps extends React.HTMLAttributes<HTMLDivElement> {
  value?: number;
  /** Transition duration in milliseconds. */
  durationMs?: number;
}

const Progress = React.forwardRef<HTMLDivElement, ProgressProps>(
  ({ className, value = 0, durationMs = 150, ...props }, ref) => {
    return (
      <div
        ref={ref}
        className={cn(
          "relative h-2 w-full overflow-hidden rounded-full bg-secondary",
          className,
        )}
        {...props}
      >
        <div
          className="h-full bg-primary transition-all"
          style={{
            width: `${value}%`,
            transitionDuration: `${durationMs}ms`,
          }}
        />
      </div>
    );
  },
);
Progress.displayName = "Progress";

export { Progress };
