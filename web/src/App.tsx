import { useEffect, useRef, useState } from "react";
import { toPng } from "html-to-image";
import { bury, health, type Result, type Step } from "./api";
import { notAPhoto, prepare, slug } from "./image";
import { Card, HowItKnows, NoticeCard } from "./Card";

type State =
  | { kind: "idle" }
  | { kind: "working"; step: Step; started: number }
  | { kind: "done"; result: Result; print: string }
  | { kind: "error"; message: string };

const stepText: Record<Step, string> = {
  look: "Examining the remains…",
  write: "Writing the obituary…",
};

const isPhone = typeof window !== "undefined" && window.matchMedia("(pointer: coarse)").matches;

export function App() {
  const [state, setState] = useState<State>({ kind: "idle" });
  const [warning, setWarning] = useState("");
  const [dragging, setDragging] = useState(false);
  const [saving, setSaving] = useState(false);
  const fileInput = useRef<HTMLInputElement>(null);
  const cameraInput = useRef<HTMLInputElement>(null);
  const firstButton = useRef<HTMLButtonElement>(null);
  const cardRef = useRef<HTMLElement>(null);
  const resultRef = useRef<HTMLElement>(null);

  useEffect(() => {
    health().then((h) => {
      if (!h) setWarning("Can't reach Here Lies. Is the server still running? Start it with: make run");
      else if (h.problem) setWarning(h.problem);
    });
  }, []);

  // Move the reader to the result once it is on the page.
  useEffect(() => {
    if (state.kind !== "done") return;
    const reduce = window.matchMedia("(prefers-reduced-motion: reduce)").matches;
    resultRef.current?.scrollIntoView({ behavior: reduce ? "auto" : "smooth", block: "start" });
    document.querySelector<HTMLElement>("#obit-name, #notice-title")?.focus({ preventScroll: true });
  }, [state]);

  async function handle(file: File | undefined) {
    if (!file || state.kind === "working") return;
    if (file.type && !file.type.startsWith("image/")) {
      setState({ kind: "error", message: notAPhoto });
      return;
    }
    setState({ kind: "working", step: "look", started: Date.now() });
    try {
      const photo = await prepare(file);
      const started = Date.now();
      const result = await bury(photo.upload, (step) => setState({ kind: "working", step, started }));
      setWarning("");
      setState({ kind: "done", result, print: photo.print });
    } catch (e) {
      setState({ kind: "error", message: e instanceof Error ? e.message : String(e) });
    }
  }

  function again() {
    setState({ kind: "idle" });
    window.scrollTo({ top: 0 });
    firstButton.current?.focus();
  }

  async function download() {
    if (state.kind !== "done" || !state.result.obituary || !cardRef.current) return;
    setSaving(true);
    const card = cardRef.current;
    card.classList.add("exporting");
    try {
      const url = await toPng(card, {
        pixelRatio: 2,
        cacheBust: false,
        style: { transform: "none", margin: "0" },
      });
      const a = document.createElement("a");
      a.href = url;
      a.download = `here-lies-${slug(state.result.obituary.name)}.png`;
      document.body.appendChild(a);
      a.click();
      a.remove();
    } catch {
      setState({ kind: "error", message: "The card couldn't be saved as a picture. Try again." });
    } finally {
      card.classList.remove("exporting");
      setSaving(false);
    }
  }

  const working = state.kind === "working";

  return (
    <main className="page">
      <section className="intro">
        <h1>Here Lies</h1>
        <p>Take a photo of something that died in your house. A model on this computer writes its obituary.</p>
      </section>

      <section
        className={`drop${dragging ? " dragging" : ""}`}
        aria-label="Add a photo"
        onDragOver={(e) => {
          e.preventDefault();
          setDragging(true);
        }}
        onDragLeave={() => setDragging(false)}
        onDrop={(e) => {
          e.preventDefault();
          setDragging(false);
          handle(e.dataTransfer.files[0]);
        }}
      >
        <strong>{isPhone ? "Photograph the deceased" : "Drop a photo of the deceased here"}</strong>
        <span>Dead earbuds, a cracked phone, a sock with a hole, last night's pumpkin.</span>
        <div className="drop-actions">
          {isPhone && (
            <button className="btn" type="button" ref={firstButton} disabled={working} onClick={() => cameraInput.current?.click()}>
              Take a photo
            </button>
          )}
          <button
            className={isPhone ? "btn ghost" : "btn"}
            type="button"
            ref={isPhone ? undefined : firstButton}
            disabled={working}
            onClick={() => fileInput.current?.click()}
          >
            Choose a file
          </button>
        </div>
        <input
          ref={fileInput}
          id="photo-file"
          type="file"
          accept="image/*"
          hidden
          onChange={(e) => {
            handle(e.target.files?.[0]);
            e.target.value = "";
          }}
        />
        <input
          ref={cameraInput}
          type="file"
          accept="image/*"
          capture="environment"
          hidden
          onChange={(e) => {
            handle(e.target.files?.[0]);
            e.target.value = "";
          }}
        />
        <div className="privacy">Your photo stays on this computer. Nothing is saved.</div>
        {warning && state.kind !== "error" && (
          <p className="error" role="alert">
            {warning}
          </p>
        )}
        {state.kind === "error" && (
          <p className="error" role="alert">
            {state.message}
          </p>
        )}
      </section>

      <div className="progress" aria-live="polite">
        {working && <Progress step={state.step} started={state.started} />}
      </div>

      {state.kind === "done" && (
        <section className="result" aria-label="Result" ref={resultRef}>
          {state.result.kind === "obituary" && state.result.obituary ? (
            <>
              <div>
                <div className="clip-wrap">
                  <Card
                    ref={cardRef}
                    obit={state.result.obituary}
                    details={state.result.details}
                    print={state.print}
                    object={state.result.object}
                  />
                </div>
                <div className="card-actions">
                  <button className="btn" type="button" onClick={download} disabled={saving}>
                    {saving ? "Saving…" : "Download card"}
                  </button>
                  <button className="btn ghost" type="button" onClick={again}>
                    Bury another
                  </button>
                </div>
              </div>
              <HowItKnows
                details={state.result.details}
                natural={state.result.obituary.cause.natural}
                repaired={state.result.checks?.repaired ?? []}
              />
            </>
          ) : (
            state.result.notice && (
              <div>
                <NoticeCard notice={state.result.notice} />
                <div className="card-actions">
                  <button className="btn" type="button" onClick={again}>
                    Try another photo
                  </button>
                </div>
              </div>
            )
          )}
        </section>
      )}
    </main>
  );
}

function Progress({ step, started }: { step: Step; started: number }) {
  const [now, setNow] = useState(Date.now());
  useEffect(() => {
    const t = setInterval(() => setNow(Date.now()), 500);
    return () => clearInterval(t);
  }, []);
  return (
    <>
      <span>{stepText[step]}</span>
      <span>{Math.max(0, Math.floor((now - started) / 1000))} s</span>
    </>
  );
}
