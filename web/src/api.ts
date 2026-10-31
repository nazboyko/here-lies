export type Detail = {
  id: string;
  text: string;
  kind: "damage" | "wear" | "feature";
  used_in: string[];
};

export type Obituary = {
  name: string;
  born: string;
  died: string;
  dateline: string;
  headline: string;
  cause: { text: string; detail_id?: string; natural: boolean };
  life: { text: string; detail_ids: string[] }[];
  survived_by: string;
  in_lieu_of_flowers: string;
  epitaph: string;
};

export type Notice = { subject: string; title: string; line: string };

export type Checks = {
  first_try_pass: boolean;
  retried: boolean;
  fallback_used: boolean;
  soft_mismatches: number;
  repaired: string[];
};

export type Result = {
  kind: "obituary" | "notice";
  object?: string;
  obituary?: Obituary;
  notice?: Notice;
  details: Detail[];
  checks?: Checks;
  timings: { look_ms: number; write_ms: number; total_ms: number };
};

export type Health = { reachable: boolean; model_present: boolean; model: string; problem?: string };

export type Step = "look" | "write";

const offline = "Can't reach Here Lies. Is the server still running? Start it with: make run";

export async function health(): Promise<Health | null> {
  try {
    const res = await fetch("/api/health");
    return (await res.json()) as Health;
  } catch {
    return null;
  }
}

// bury sends the photo and reports each step as the server starts it.
export async function bury(photo: Blob, onStep: (s: Step) => void, signal?: AbortSignal): Promise<Result> {
  const form = new FormData();
  form.append("photo", photo, "photo.jpg");
  let res: Response;
  try {
    res = await fetch("/api/obituary", {
      method: "POST",
      body: form,
      headers: { Accept: "application/x-ndjson" },
      signal,
    });
  } catch (e) {
    if (signal?.aborted) throw e;
    throw new Error(offline);
  }
  if (!res.ok || !res.body) {
    let msg = offline;
    try {
      msg = ((await res.json()) as { error?: string }).error || msg;
    } catch {
      /* keep the generic message */
    }
    throw new Error(msg);
  }

  const reader = res.body.pipeThrough(new TextDecoderStream()).getReader();
  try {
    return await readEvents(reader, onStep);
  } catch (e) {
    reader.cancel().catch(() => {});
    throw e;
  }
}

async function readEvents(reader: ReadableStreamDefaultReader<string>, onStep: (s: Step) => void): Promise<Result> {
  let buf = "";
  for (;;) {
    const { value, done } = await reader.read();
    if (value) buf += value;
    let nl: number;
    while ((nl = buf.indexOf("\n")) >= 0) {
      const line = buf.slice(0, nl).trim();
      buf = buf.slice(nl + 1);
      if (!line) continue;
      const ev = JSON.parse(line) as { step?: Step; result?: Result; error?: string };
      if (ev.error) throw new Error(ev.error);
      if (ev.result) return ev.result;
      if (ev.step) onStep(ev.step);
    }
    if (done) break;
  }
  throw new Error("The obituary was cut off. Try again.");
}
