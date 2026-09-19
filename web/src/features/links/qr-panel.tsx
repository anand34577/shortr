import * as React from "react";
import { QRCodeSVG } from "qrcode.react";
import { Download } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Label } from "@/components/ui/label";
import { Input } from "@/components/ui/input";
import type { Link } from "@/lib/types";

export function QrPanel({ link }: { link: Link }) {
  const [size, setSize] = React.useState(256);
  const [fg, setFg] = React.useState("#18181b");
  const [bg, setBg] = React.useState("#ffffff");
  const svgRef = React.useRef<SVGSVGElement>(null);

  function downloadSvg() {
    const svg = svgRef.current;
    if (!svg) return;
    const serializer = new XMLSerializer();
    const source = serializer.serializeToString(svg);
    const blob = new Blob([source], { type: "image/svg+xml" });
    const url = URL.createObjectURL(blob);
    const a = document.createElement("a");
    a.href = url;
    a.download = `${link.code}-qr.svg`;
    a.click();
    URL.revokeObjectURL(url);
  }

  function downloadPng() {
    const svg = svgRef.current;
    if (!svg) return;
    const serializer = new XMLSerializer();
    const source = serializer.serializeToString(svg);
    const img = new Image();
    const svgBlob = new Blob([source], { type: "image/svg+xml;charset=utf-8" });
    const url = URL.createObjectURL(svgBlob);
    img.onload = () => {
      const canvas = document.createElement("canvas");
      canvas.width = size;
      canvas.height = size;
      const ctx = canvas.getContext("2d");
      if (ctx) {
        ctx.fillStyle = bg;
        ctx.fillRect(0, 0, size, size);
        ctx.drawImage(img, 0, 0, size, size);
      }
      URL.revokeObjectURL(url);
      canvas.toBlob((blob) => {
        if (!blob) return;
        const pngUrl = URL.createObjectURL(blob);
        const a = document.createElement("a");
        a.href = pngUrl;
        a.download = `${link.code}-qr.png`;
        a.click();
        URL.revokeObjectURL(pngUrl);
      });
    };
    img.src = url;
  }

  return (
    <div className="flex flex-col items-center gap-4 sm:flex-row sm:items-start">
      <div className="flex items-center justify-center rounded-xl border border-border p-4" style={{ background: bg }}>
        <QRCodeSVG ref={svgRef} value={link.shortUrl} size={size} fgColor={fg} bgColor={bg} level="M" includeMargin />
      </div>
      <div className="flex flex-1 flex-col gap-3">
        <div className="flex flex-col gap-1.5">
          <Label htmlFor="qr-size">Size ({size}px)</Label>
          <input
            id="qr-size"
            type="range"
            min={128}
            max={512}
            step={16}
            value={size}
            onChange={(e) => setSize(Number(e.target.value))}
          />
        </div>
        <div className="flex gap-3">
          <div className="flex flex-1 flex-col gap-1.5">
            <Label htmlFor="qr-fg">Foreground</Label>
            <Input id="qr-fg" type="color" value={fg} onChange={(e) => setFg(e.target.value)} className="h-9 p-1" />
          </div>
          <div className="flex flex-1 flex-col gap-1.5">
            <Label htmlFor="qr-bg">Background</Label>
            <Input id="qr-bg" type="color" value={bg} onChange={(e) => setBg(e.target.value)} className="h-9 p-1" />
          </div>
        </div>
        <div className="flex gap-2">
          <Button variant="outline" onClick={downloadPng} className="flex-1 gap-2">
            <Download className="size-4" /> PNG
          </Button>
          <Button variant="outline" onClick={downloadSvg} className="flex-1 gap-2">
            <Download className="size-4" /> SVG
          </Button>
        </div>
      </div>
    </div>
  );
}
