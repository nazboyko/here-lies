import { forwardRef } from "react";
import type { Detail, Notice, Obituary } from "./api";

type CardProps = { obit: Obituary; details: Detail[]; print: string; object?: string };

export const Card = forwardRef<HTMLElement, CardProps>(function Card({ obit, details, print, object }, ref) {
  return (
    <article className="clipping" id="card" ref={ref} aria-labelledby="obit-name">
      <span className="tape tl" aria-hidden="true" />
      <span className="tape tr" aria-hidden="true" />

      <h2 className="mast">The Household Gazette</h2>
      <div className="dateline">
        <span>Obituaries</span>
        <span>{obit.dateline}</span>
      </div>

      <figure className="photo">
        <img src={print} alt={object ? `The deceased: ${object}` : "The deceased"} />
      </figure>

      <h3 className="name" id="obit-name" tabIndex={-1}>
        {obit.name}
      </h3>
      <div className="years">
        {obit.born} – {obit.died}
      </div>
      {obit.headline && <p className="deck">{obit.headline}</p>}

      <p>
        <b>Cause of death:</b> {obit.cause.text}
        {obit.cause.natural && <span className="no-injuries"> No visible injuries.</span>}
      </p>
      <p className="life">
        {obit.life.map((s, i) => (
          <span key={i} data-cites={s.detail_ids.join(" ")}>
            {i > 0 && " "}
            {s.text}
          </span>
        ))}
      </p>
      <p>
        <b>Survived by</b> {obit.survived_by}
      </p>
      <p>
        <b>In lieu of flowers,</b> {obit.in_lieu_of_flowers}
      </p>

      <Marigolds />

      <p className="epitaph">“{obit.epitaph}”</p>
      <p className="seen">Based on what we could see: {details.map((d) => d.text).join("; ")}.</p>
    </article>
  );
});

export function NoticeCard({ notice }: { notice: Notice }) {
  return (
    <div className="notice" role="status">
      <h2 className="mast">The Household Gazette</h2>
      <h3 tabIndex={-1} id="notice-title">
        {notice.title}
      </h3>
      <p>{notice.line}</p>
    </div>
  );
}

function Marigolds() {
  const xs = [150, 175, 200, 225, 250];
  return (
    <svg className="marigolds" viewBox="0 0 400 16" preserveAspectRatio="xMidYMid meet" aria-hidden="true">
      <line x1="0" y1="8" x2="400" y2="8" stroke="#24201c" strokeWidth=".8" />
      {xs.map((x) => (
        <g key={x} transform={`translate(${x} 8)`}>
          <circle r="5.5" fill="#f2a20c" />
          <circle r="3.2" fill="#c9640a" />
          <circle r="1.2" fill="#24201c" />
        </g>
      ))}
    </svg>
  );
}

export function HowItKnows({ details, natural, repaired }: { details: Detail[]; natural: boolean; repaired: string[] }) {
  return (
    <aside className="side" aria-labelledby="how-title">
      <h2 id="how-title">How does it know?</h2>
      <p>The name, the dates and the jokes are made up. The physical details are not.</p>

      <details className="know" open>
        <summary>What the model saw in the photo</summary>
        <ul className="details-list">
          {details.map((d) => (
            <li key={d.id} data-id={d.id}>
              {d.used_in.length > 0 ? (
                <span className="ok" aria-label="used on the card">
                  ✓
                </span>
              ) : (
                <span className="unused" aria-label="not used on the card">
                  –
                </span>
              )}
              <div>
                {capitalize(d.text)}
                <span className="used">
                  {d.used_in.length > 0 ? `Used in: ${d.used_in.join("; ")}` : "Not used on the card"}
                </span>
              </div>
            </li>
          ))}
        </ul>
        <p className="rule-note">
          Every sentence about how the object looks has to point at one of these. A plain check in the code, not the
          model, makes sure it does.
        </p>
        {natural && <p className="rule-note">No visible injuries, so the cause of death is natural causes.</p>}
        {repaired.length > 0 && (
          <div className="repaired">
            <strong>Fixed by the check</strong>
            <ul>
              {repaired.map((r) => (
                <li key={r}>{r}</li>
              ))}
            </ul>
          </div>
        )}
      </details>
    </aside>
  );
}

function capitalize(s: string) {
  return s.charAt(0).toUpperCase() + s.slice(1);
}
