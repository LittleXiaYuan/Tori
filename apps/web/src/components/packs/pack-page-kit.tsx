"use client";

/**
 * Shared Pro-styled building blocks for capability-pack pages.
 *
 * Replaces the per-page copy-pasted skeleton (intro card / boundary grid /
 * steps / developer detail dump) that leaned on inline `var(--yunque-*)`
 * styling and high information density. These use HeroUI v3 semantic
 * components (Card variants, Chip colors, Accordion) and HeroUI utility
 * tokens (text-foreground / text-muted / bg-surface-secondary) which are
 * bridged to the app theme in globals.css, so they theme correctly without
 * hand-rolled inline colors.
 */

import type { ReactNode } from "react";
import { Card, Chip, Accordion, Separator } from "@heroui/react";

export type Tone = "neutral" | "success" | "warning" | "danger" | "accent";

const toneText: Record<Tone, string> = {
  neutral: "text-foreground",
  success: "text-success",
  warning: "text-warning",
  danger: "text-danger",
  accent: "text-accent",
};

/** Page section heading with an icon — Title Case, no ALL CAPS, no heavy borders. */
export function PackSectionTitle({ icon, children, tone = "neutral" }: { icon?: ReactNode; children: ReactNode; tone?: Tone }) {
  return (
    <div className="flex items-center gap-2">
      {icon ? <span className={`shrink-0 ${toneText[tone]}`}>{icon}</span> : null}
      <span className="text-sm font-semibold text-foreground">{children}</span>
    </div>
  );
}

/**
 * PackHero — the primary "what this pack does" surface: status chips, a clear
 * title, a concise description (capped to a few lines), optional note banner,
 * example chips, and a trailing action cluster. Generous padding, one card.
 */
export function PackHero({
  chips,
  title,
  description,
  note,
  examples,
  actions,
}: {
  chips?: ReactNode;
  title: string;
  description?: ReactNode;
  note?: ReactNode;
  examples?: string[];
  actions?: ReactNode;
}) {
  return (
    <Card variant="default">
      <Card.Header className="gap-3">
        {chips ? <div className="flex flex-wrap items-center gap-2">{chips}</div> : null}
        <Card.Title className="text-lg">{title}</Card.Title>
        {description ? (
          <Card.Description className="max-w-3xl text-sm leading-6 text-muted">{description}</Card.Description>
        ) : null}
      </Card.Header>
      {(note || (examples && examples.length > 0)) && (
        <Card.Content className="flex flex-col gap-4">
          {note ? (
            <div className="rounded-xl bg-surface-secondary px-4 py-3 text-sm leading-6 text-muted">{note}</div>
          ) : null}
          {examples && examples.length > 0 ? (
            <div className="grid gap-3 md:grid-cols-3">
              {examples.map((ex) => (
                <div key={ex} className="rounded-xl bg-surface-secondary px-4 py-3 text-sm leading-6 text-muted">
                  {ex}
                </div>
              ))}
            </div>
          ) : null}
        </Card.Content>
      )}
      {actions ? <Card.Footer className="flex flex-wrap gap-2">{actions}</Card.Footer> : null}
    </Card>
  );
}

export interface PackBoundaryItem {
  key: string;
  label: string;
  detail: ReactNode;
  tone?: Tone;
}

/**
 * PackBoundaryGrid — "what this pack will NOT do" / safety boundaries. Neutral
 * surface tiles; only the label carries semantic color, per HeroUI taste
 * (keep backgrounds neutral, let accents come from text/icons).
 */
export function PackBoundaryGrid({ title, icon, items, columns = 2 }: { title: string; icon?: ReactNode; items: PackBoundaryItem[]; columns?: 2 | 3 }) {
  const grid = columns === 3 ? "md:grid-cols-3" : "md:grid-cols-2";
  return (
    <Card variant="secondary">
      <Card.Header>
        <PackSectionTitle icon={icon} tone="accent">{title}</PackSectionTitle>
      </Card.Header>
      <Card.Content>
        <div className={`grid gap-3 ${grid}`}>
          {items.map((item) => (
            <div key={item.key} className="rounded-xl bg-surface-secondary px-4 py-3 text-sm leading-6 text-muted">
              <div className={`mb-1 font-semibold ${toneText[item.tone ?? "neutral"]}`}>{item.label}</div>
              {item.detail}
            </div>
          ))}
        </div>
      </Card.Content>
    </Card>
  );
}

export interface PackStep {
  key: string;
  label: string;
  detail: ReactNode;
}

/** PackStepsGrid — numbered "how to use / verify" steps in a calm grid. */
export function PackStepsGrid({ steps, columns = 3 }: { steps: PackStep[]; columns?: 2 | 3 | 4 }) {
  const grid = columns === 4 ? "md:grid-cols-4" : columns === 2 ? "md:grid-cols-2" : "md:grid-cols-3";
  return (
    <div className={`grid gap-3 ${grid}`}>
      {steps.map((step, idx) => (
        <div key={step.key} className="rounded-xl bg-surface-secondary px-4 py-3 text-sm leading-6 text-muted">
          <div className="mb-1.5 flex items-center gap-2 font-semibold text-foreground">
            <span className="inline-flex size-5 items-center justify-center rounded-full bg-accent/15 text-xs text-accent">
              {idx + 1}
            </span>
            {step.label}
          </div>
          {step.detail}
        </div>
      ))}
    </div>
  );
}

export interface PackInfoSection {
  key: string;
  icon?: ReactNode;
  title: string;
  body: ReactNode;
}

/**
 * PackInfoAccordion — progressive disclosure for developer / sync / SDK detail
 * that used to dominate the page. Collapsed by default to cut visual density.
 */
export function PackInfoAccordion({ sections }: { sections: PackInfoSection[] }) {
  if (sections.length === 0) return null;
  return (
    <Card variant="transparent" className="p-0">
      <Accordion className="w-full">
        {sections.map((s) => (
          <Accordion.Item key={s.key}>
            <Accordion.Heading>
              <Accordion.Trigger className="text-sm font-semibold text-foreground">
                {s.icon ? <span className="mr-2 inline-flex size-4 shrink-0 text-muted">{s.icon}</span> : null}
                {s.title}
                <Accordion.Indicator />
              </Accordion.Trigger>
            </Accordion.Heading>
            <Accordion.Panel>
              <Accordion.Body className="text-sm leading-6 text-muted">{s.body}</Accordion.Body>
            </Accordion.Panel>
          </Accordion.Item>
        ))}
      </Accordion>
    </Card>
  );
}

/**
 * PackAbout — collapsed-by-default "what is this pack / boundaries" meta. Keeps
 * the generic boilerplate (status chips, one-line purpose, safety boundaries)
 * available without letting it dominate the top of every page or make every
 * pack page look identical. Each page should LEAD with its own unique content
 * and tuck this generic context behind the disclosure.
 */
export function PackAbout({
  title = "关于这个能力包",
  chips,
  description,
  boundaries,
}: {
  title?: string;
  chips?: ReactNode;
  description?: ReactNode;
  boundaries?: PackBoundaryItem[];
}) {
  return (
    <Card variant="transparent" className="p-0">
      <Accordion className="w-full">
        <Accordion.Item>
          <Accordion.Heading>
            <Accordion.Trigger className="text-sm font-semibold text-foreground">
              {title}
              <Accordion.Indicator />
            </Accordion.Trigger>
          </Accordion.Heading>
          <Accordion.Panel>
            <Accordion.Body className="flex flex-col gap-4">
              {chips ? <div className="flex flex-wrap items-center gap-2">{chips}</div> : null}
              {description ? <p className="max-w-3xl text-sm leading-6 text-muted">{description}</p> : null}
              {boundaries && boundaries.length > 0 ? (
                <div className="flex flex-col gap-2">
                  <div className="text-xs font-semibold text-muted">当前不会做什么</div>
                  <div className="grid gap-3 md:grid-cols-2">
                    {boundaries.map((b) => (
                      <div key={b.key} className="rounded-xl bg-surface-secondary px-4 py-3 text-sm leading-6 text-muted">
                        <div className={`mb-1 font-semibold ${toneText[b.tone ?? "neutral"]}`}>{b.label}</div>
                        {b.detail}
                      </div>
                    ))}
                  </div>
                </div>
              ) : null}
            </Accordion.Body>
          </Accordion.Panel>
        </Accordion.Item>
      </Accordion>
    </Card>
  );
}

/** Inline labelled value used inside disclosure panels (replaces dense code rows). */
export function PackKeyValue({ label, children }: { label: string; children: ReactNode }) {
  return (
    <div className="flex flex-wrap items-baseline gap-2 py-1">
      <span className="text-xs text-muted">{label}</span>
      <span className="font-mono text-xs text-foreground">{children}</span>
    </div>
  );
}

export { Separator as PackSeparator, Chip as PackChip };
