const maxSide = 1024;
export const notAPhoto = "This file isn't a photo we can read. Use a JPEG, PNG or WebP picture.";

export type Prepared = {
  upload: Blob; // JPEG for the model, at most 1024 px, no EXIF
  print: string; // grayscale newsprint version for the card, as a data URL
};

// prepare decodes the photo upright, scales it down, and makes both the
// upload and the newsprint print. Drawing to a canvas drops EXIF data.
export async function prepare(file: File): Promise<Prepared> {
  let bitmap: ImageBitmap;
  try {
    bitmap = await createImageBitmap(file, { imageOrientation: "from-image" });
  } catch {
    throw new Error(notAPhoto);
  }
  const scale = Math.min(1, maxSide / Math.max(bitmap.width, bitmap.height));
  const w = Math.max(1, Math.round(bitmap.width * scale));
  const h = Math.max(1, Math.round(bitmap.height * scale));
  const canvas = document.createElement("canvas");
  canvas.width = w;
  canvas.height = h;
  const ctx = canvas.getContext("2d");
  if (!ctx) throw new Error(notAPhoto);
  ctx.fillStyle = "#fff";
  ctx.fillRect(0, 0, w, h);
  ctx.drawImage(bitmap, 0, 0, w, h);
  bitmap.close();

  const upload = await new Promise<Blob>((resolve, reject) =>
    canvas.toBlob((b) => (b ? resolve(b) : reject(new Error(notAPhoto))), "image/jpeg", 0.85),
  );

  newsprint(ctx, w, h);
  return { upload, print: canvas.toDataURL("image/jpeg", 0.88) };
}

// newsprint turns the pixels gray with a faint warm tint and a little
// contrast, like a photo in an old paper. Done by hand so the downloaded
// PNG looks exactly like the card on screen.
function newsprint(ctx: CanvasRenderingContext2D, w: number, h: number) {
  const img = ctx.getImageData(0, 0, w, h);
  const p = img.data;
  for (let i = 0; i < p.length; i += 4) {
    let y = 0.299 * p[i] + 0.587 * p[i + 1] + 0.114 * p[i + 2];
    y = Math.min(255, Math.max(0, (y - 128) * 1.08 + 128));
    p[i] = Math.min(255, y * 1.04 + 4);
    p[i + 1] = Math.min(255, y * 1.0 + 2);
    p[i + 2] = y * 0.9;
  }
  ctx.putImageData(img, 0, 0);
}

export function slug(name: string): string {
  return (
    name
      .toLowerCase()
      .normalize("NFKD")
      .replace(/[^a-z0-9]+/g, "-")
      .replace(/^-+|-+$/g, "")
      .slice(0, 40) || "obituary"
  );
}
