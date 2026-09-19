import { format, formatDistanceToNow, parseISO } from "date-fns";
import { Tooltip, TooltipContent, TooltipTrigger, TooltipProvider } from "@/components/ui/tooltip";

/** Displays a timestamp in the viewer's local timezone with a UTC tooltip. */
export function LocalTime({
  iso,
  relative = false,
  className,
}: {
  iso: string | null | undefined;
  relative?: boolean;
  className?: string;
}) {
  if (!iso) return <span className={className}>—</span>;
  let date: Date;
  try {
    date = parseISO(iso);
  } catch {
    return <span className={className}>{iso}</span>;
  }
  if (Number.isNaN(date.getTime())) return <span className={className}>{iso}</span>;

  const label = relative ? formatDistanceToNow(date, { addSuffix: true }) : format(date, "PP p");
  const utc = date.toUTCString();

  return (
    <TooltipProvider delayDuration={300}>
      <Tooltip>
        <TooltipTrigger asChild>
          <time dateTime={iso} className={className}>
            {label}
          </time>
        </TooltipTrigger>
        <TooltipContent>{utc}</TooltipContent>
      </Tooltip>
    </TooltipProvider>
  );
}
