import * as React from "react";

/**
 * Cursor paging with a back stack. The first page has no cursor, so it is
 * stored as "" on the stack; skipping falsy cursors would lose page 1 and
 * leave "Previous" disabled on page 2.
 */
export function useCursorPager() {
  const [cursor, setCursor] = React.useState<string | undefined>();
  const [stack, setStack] = React.useState<string[]>([]);

  return {
    cursor,
    /** 1-based page number */
    page: stack.length + 1,
    hasPrevious: stack.length > 0,
    next(nextCursor: string) {
      setStack((s) => [...s, cursor ?? ""]);
      setCursor(nextCursor);
    },
    previous() {
      if (stack.length === 0) return;
      setCursor(stack[stack.length - 1] || undefined);
      setStack((s) => s.slice(0, -1));
    },
    reset() {
      setCursor(undefined);
      setStack([]);
    },
  };
}
