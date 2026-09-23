import type { UseFormSetError, FieldValues, Path } from "react-hook-form";
import { ApiError } from "./api";
import { toast } from "sonner";

/**
 * Maps the API error envelope's `fields` map onto react-hook-form field
 * errors, so they show next to the field rather than only in a toast.
 * Falls back to a toast for non-field errors.
 */
export function applyServerErrors<T extends FieldValues>(
  err: unknown,
  setError: UseFormSetError<T>,
): void {
  if (err instanceof ApiError) {
    if (err.fields && Object.keys(err.fields).length > 0) {
      for (const [field, message] of Object.entries(err.fields)) {
        // the API reports snake_case field names; forms use camelCase
        const name = field.replace(/_([a-z])/g, (_, c: string) => c.toUpperCase());
        setError(name as Path<T>, { type: "server", message });
      }
      return;
    }
  }
  toastError(err);
}

/**
 * Error feedback for actions that aren't forms (menu items, toggles,
 * one-click buttons). `fallback` is used when the server gave no message.
 */
export function toastError(err: unknown, fallback = "Something went wrong. Please try again.") {
  toast.error(err instanceof ApiError ? err.message || fallback : fallback);
}
