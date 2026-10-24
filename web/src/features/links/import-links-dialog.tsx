import * as React from "react";
import { toast } from "sonner";
import { Copy, FileUp, Loader2 } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Label } from "@/components/ui/label";
import { Textarea } from "@/components/ui/textarea";
import { TagInput } from "@/components/ui/tag-input";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { useImportLinks, type BulkResult, type ImportRow } from "@/features/links/api";
import { copyText } from "@/lib/clipboard";
import { toastError } from "@/lib/apply-server-errors";

const MAX_ROWS = 1000;

/**
 * One link per line: `url`, `url,alias` or `url,alias,title` (tab-separated
 * also works, so a spreadsheet column paste just works). Blank lines, `#`
 * comments and a leading `url,...` header row are skipped.
 */
function parseLines(text: string): { line: number; row: ImportRow }[] {
  const out: { line: number; row: ImportRow }[] = [];
  text.split(/\r?\n/).forEach((raw, i) => {
    const s = raw.trim();
    if (!s || s.startsWith("#")) return;
    const [url, code, ...title] = s.split(s.includes("\t") ? "\t" : ",").map((f) => f.trim());
    if (out.length === 0 && /^(url|target_?url)$/i.test(url)) return;
    out.push({ line: i + 1, row: { targetUrl: url, code: code || undefined, title: title.join(", ") || undefined } });
  });
  return out;
}

export function ImportLinksDialog({ open, onOpenChange }: { open: boolean; onOpenChange: (open: boolean) => void }) {
  const [text, setText] = React.useState("");
  const [tags, setTags] = React.useState<string[]>([]);
  const [results, setResults] = React.useState<{ line: number; res: BulkResult }[] | null>(null);
  const fileRef = React.useRef<HTMLInputElement>(null);
  const importLinks = useImportLinks();
  const rows = React.useMemo(() => parseLines(text), [text]);

  React.useEffect(() => {
    if (open) {
      setText("");
      setTags([]);
      setResults(null);
    }
  }, [open]);

  async function onFile(e: React.ChangeEvent<HTMLInputElement>) {
    const file = e.target.files?.[0];
    e.target.value = ""; // allow picking the same file again
    if (file) setText(await file.text());
  }

  async function submit() {
    try {
      const res = await importLinks.mutateAsync(rows.map(({ row }) => ({ ...row, tags: tags.length ? tags : undefined })));
      const mapped = res.map((r, i) => ({ line: rows[i].line, res: r }));
      setResults(mapped);
      const ok = res.filter((r) => r.ok).length;
      if (ok) toast.success(`Imported ${ok} link${ok === 1 ? "" : "s"}`);
    } catch (err) {
      toastError(err, "Import failed");
    }
  }

  const created = results?.filter((r) => r.res.ok) ?? [];
  const failed = results?.filter((r) => !r.res.ok) ?? [];

  async function copyCreated() {
    const copied = await copyText(created.map((r) => r.res.shortUrl).join("\n"));
    toast[copied ? "success" : "error"](copied ? "Short links copied" : "Copy failed");
  }

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="sm:max-w-xl">
        <DialogHeader>
          <DialogTitle>Import links</DialogTitle>
          <DialogDescription>
            One link per line: <code className="font-mono text-xs">url</code>,{" "}
            <code className="font-mono text-xs">url,alias</code> or{" "}
            <code className="font-mono text-xs">url,alias,title</code>. Paste from a spreadsheet or load a CSV file.
          </DialogDescription>
        </DialogHeader>

        {results ? (
          <div className="flex flex-col gap-3 text-sm" aria-live="polite">
            <p>
              <span className="font-medium">{created.length}</span> created
              {failed.length > 0 && (
                <>
                  , <span className="font-medium text-destructive">{failed.length}</span> failed
                </>
              )}
              .
            </p>
            {failed.length > 0 && (
              <ul className="max-h-48 overflow-y-auto rounded-lg border border-border p-2 text-xs">
                {failed.map((r) => (
                  <li key={r.line} className="py-0.5">
                    <span className="font-mono text-muted-foreground">Line {r.line}:</span> {r.res.error}
                  </li>
                ))}
              </ul>
            )}
          </div>
        ) : (
          <div className="flex flex-col gap-3">
            <div className="flex flex-col gap-1.5">
              <div className="flex items-center justify-between">
                <Label htmlFor="import-lines">Links</Label>
                <Button type="button" variant="ghost" size="sm" className="gap-1.5" onClick={() => fileRef.current?.click()}>
                  <FileUp className="size-4" /> Load file
                </Button>
                <input ref={fileRef} type="file" accept=".csv,.tsv,.txt,text/csv,text/plain" className="hidden" onChange={onFile} />
              </div>
              <Textarea
                id="import-lines"
                value={text}
                onChange={(e) => setText(e.target.value)}
                placeholder={"https://example.com/spring-sale,spring,Spring sale\nhttps://example.com/blog/post"}
                className="min-h-40 font-mono text-xs"
                spellCheck={false}
              />
              <p className="text-xs text-muted-foreground">
                {rows.length} link{rows.length === 1 ? "" : "s"} detected
                {rows.length > MAX_ROWS && ` · at most ${MAX_ROWS} per import`}
              </p>
            </div>
            <div className="flex flex-col gap-1.5">
              <Label htmlFor="import-tags">Tags for every link (optional)</Label>
              <TagInput id="import-tags" value={tags} onChange={setTags} />
            </div>
          </div>
        )}

        <DialogFooter>
          {results ? (
            <>
              {created.length > 0 && (
                <Button variant="outline" className="gap-1.5" onClick={copyCreated}>
                  <Copy className="size-4" /> Copy short links
                </Button>
              )}
              <Button onClick={() => onOpenChange(false)}>Done</Button>
            </>
          ) : (
            <>
              <Button variant="outline" onClick={() => onOpenChange(false)}>
                Cancel
              </Button>
              <Button
                onClick={submit}
                disabled={rows.length === 0 || rows.length > MAX_ROWS || importLinks.isPending}
                className="gap-1.5"
              >
                {importLinks.isPending && <Loader2 className="size-4 animate-spin" />}
                Import {rows.length > 0 ? rows.length : ""}
              </Button>
            </>
          )}
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
