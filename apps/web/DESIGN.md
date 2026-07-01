# Glass Design Constitution (HeroUI Pro)

> Source: HeroUI Pro Theme Builder — "Glass" theme export.
> This file is the **design reference** for the ongoing HeroUI Pro UI migration.
> The app keeps its own runtime theme engine (`src/lib/theme-engine.ts`) and the
> `--yunque-*` token bridge in `globals.css`. Use the semantic token names below
> as implementation handles — the raw hex values are for QA/handoff only.

## Core Rule

Use **semantic HeroUI tokens and Tailwind utilities** in product code, never raw
hex/rgba when a token exists. Use the same token across light and dark mode and
let CSS resolve the mode — do not branch component code to pick colors.

## Token Map (resolved values for reference)

| Token | Light | Dark | HeroUI var | Purpose |
| --- | --- | --- | --- | --- |
| background | `#F4F5F7` | `#0A0B0C` | `--background` | Page base canvas |
| foreground | `#18181B` | `#FCFCFC` | `--foreground` | Primary text/icon |
| muted | `#707276` | `#9DA0A4` | `--muted` | Secondary text, placeholders |
| surface | `#FFFFFF` | `rgba(255,255,255,0.04)` | `--surface` | Cards, panels, dropdowns |
| surface-secondary | — | — | `--surface-secondary` | Nested containers |
| surface-tertiary | — | — | `--surface-tertiary` | Deeper nesting |
| overlay | `#FFFFFF` | `rgba(255,255,255,0.05)` | `--overlay` | Modals, popovers, floating |
| accent | `#303337` | `#F8F8F9` | `--accent` | Primary actions, emphasis |
| accent-foreground | `#FCFCFC` | `#18181B` | `--accent-foreground` | Text on accent |
| accent-soft | `rgba(38,42,46,0.15)` | `rgba(248,248,249,0.12)` | `--accent-soft` | Selected / soft emphasis |
| danger | `#FF383C` | `#DB3B3E` | `--danger` | Destructive / critical |
| success | `#17C964` | `#17C964` | `--success` | Positive / completion |
| warning | `#F5A524` | `#F7B750` | `--warning` | Caution (not destructive) |
| border | `#DDDEE0` | `#28292A` | `--border` | Default container border |
| separator | `rgba(0,0,0,0.1)` | `rgba(255,255,255,0.12)` | `--separator` | Dividers |
| field-background | `#FFFFFF` | `rgba(255,255,255,0.08)` | `--field-background` | Inputs/selects |
| field-border | `rgba(0,0,0,0.04)` | `rgba(255,255,255,0.04)` | `--field-border` | Field borders |
| chart-1..5 | blue scale | accent-derived | `--chart-1..5` | Multi-series charts (chart-3 = accent) |

## Typography (Inter)

Sizes: `xs 12/16` · `sm 14/20` · `base 16/24` · `lg 18/28` · `xl 20/28` ·
`2xl 24/32` · `3xl 30/36` · `4xl 36/40` … (Tailwind `text-*` utilities).
Use HeroUI components + Tailwind text utilities; tracking via `tracking-*`.

## Layout / Spacing

- Base spacing unit `4px` — use Tailwind `gap-4`, `p-6`, `px-8`, `space-y-4`.
- Field border width `1px`, field radius `12px`, global radius `8px`.
- Keep page/section spacing on a consistent 4px/8px rhythm; don't double parent+child padding.

## Elevation

Prefer built-in `shadow-surface` / `shadow-overlay`; avoid stacking custom shadows
on Card/overlay components.

## Glass-Specific (theme-only)

- `--glass-blur` — Light 20px / Dark 36px backdrop blur for translucent surfaces.
- `--background-gradient` — ambient page gradient (applied via theme CSS, not per component).
- Note: this app already provides platform-native blur (Tauri Vibrancy/Acrylic) +
  its own `--glass-*` tokens. Do not double up backdrop-filter on nested surfaces.

## Do

- Use semantic tokens as implementation handles; raw values are reference only.
- Same semantic token across light/dark; no per-mode branching in components.
- Spacious but controlled layouts: consistent spacing, constrained max widths.
- Clear general-to-specific hierarchy: summaries + primary actions first.
- Neutral surface tokens for containers/cards/stat pills; reserve accent + status colors for real meaning.
- Build hierarchy via surface level, spacing, type scale, content order before adding borders/decoration.
- Concise, scannable typography: short Title Case headings, muted secondary text, tabular numbers for metrics.
- Content-sized badges/chips/tags — never full-width.
- Tokenized surface/overlay shadows; align nested surfaces to the same radius scale.

## Don't

- Copy raw hex/shadow/radius/spacing into product code when a token/utility exists.
- Overuse accent, warning, decorative icons, borders, or shadows for visual interest.
- Mix inconsistent radius scales or sibling card treatments in one view.
- Use color alone for hierarchy when spacing/type/surface/order communicate it better.
- Nest visually heavy surfaces inside heavy surfaces (card-on-card) without real need.
- Stretch chips/tags full width; add redundant icons/badges/wrappers.
- Add hover/transition to non-interactive content.
