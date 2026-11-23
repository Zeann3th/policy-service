---
name: Policy Service CMS
description: Admin console for limits, fees, discounts and tax, with a passbook-style dry run.
colors:
  cover: "#0f3b30"
  cover-2: "#134a3c"
  cover-deep: "#0a2c24"
  cover-ink: "#cfe3da"
  cover-muted: "#86ad9e"
  brass: "#e3b94f"
  brass-deep: "#94711c"
  brass-tint: "#fbf3dc"
  brass-ink: "#6d5112"
  ground: "#eef3f0"
  ground-2: "#e4ece7"
  card: "#ffffff"
  sunk: "#f6f9f7"
  line: "#e1e9e4"
  line-strong: "#c8d5cd"
  ink: "#15211d"
  ink-soft: "#4b5c55"
  ink-faint: "#6c7d76"
  stamp: "#c2321f"
  stamp-tint: "#fdebe7"
  ok: "#157a52"
  ok-tint: "#e2f4ea"
  accent: "#1f8a64"
typography:
  headline:
    fontFamily: "Public Sans, Segoe UI, system-ui, sans-serif"
    fontSize: "22px"
    fontWeight: 650
    lineHeight: 1.25
    letterSpacing: "-0.01em"
  verdict:
    fontFamily: "Public Sans, Segoe UI, system-ui, sans-serif"
    fontSize: "18px"
    fontWeight: 650
    letterSpacing: "-0.01em"
  title:
    fontFamily: "Public Sans, Segoe UI, system-ui, sans-serif"
    fontSize: "15px"
    fontWeight: 650
    letterSpacing: "-0.01em"
  body:
    fontFamily: "Public Sans, Segoe UI, system-ui, sans-serif"
    fontSize: "14px"
    fontWeight: 400
    lineHeight: 1.5
    fontFeature: "tnum"
  label:
    fontFamily: "Public Sans, Segoe UI, system-ui, sans-serif"
    fontSize: "12px"
    fontWeight: 600
  column-head:
    fontFamily: "Public Sans, Segoe UI, system-ui, sans-serif"
    fontSize: "11px"
    fontWeight: 600
    letterSpacing: "0.06em"
  stamp:
    fontFamily: "Public Sans, Segoe UI, system-ui, sans-serif"
    fontSize: "20px"
    fontWeight: 800
    letterSpacing: "0.14em"
  mono:
    fontFamily: "ui-monospace, Cascadia Mono, Consolas, monospace"
    fontSize: "12px"
    lineHeight: 1.55
rounded:
  sm: "7px"
  md: "9px"
  lg: "12px"
  log: "14px"
  dialog: "16px"
  stamp: "8px"
  pill: "999px"
spacing:
  xs: "6px"
  sm: "10px"
  md: "14px"
  lg: "20px"
  xl: "24px"
  page-y: "28px"
  page-x: "36px"
components:
  button-primary:
    backgroundColor: "{colors.cover}"
    textColor: "{colors.card}"
    rounded: "{rounded.sm}"
    padding: "0 14px"
    height: "36px"
  button-primary-hover:
    backgroundColor: "{colors.cover-2}"
  button-secondary:
    backgroundColor: "{colors.card}"
    textColor: "{colors.ink}"
    rounded: "{rounded.sm}"
    padding: "0 14px"
    height: "36px"
  button-ghost:
    backgroundColor: "transparent"
    textColor: "{colors.ink}"
    rounded: "{rounded.sm}"
    padding: "0 10px"
    height: "36px"
  button-ghost-hover:
    backgroundColor: "{colors.sunk}"
  button-danger-armed:
    backgroundColor: "{colors.stamp}"
    textColor: "{colors.card}"
    rounded: "{rounded.sm}"
    height: "36px"
  input:
    backgroundColor: "{colors.card}"
    textColor: "{colors.ink}"
    rounded: "{rounded.sm}"
    padding: "0 11px"
    height: "38px"
  card:
    backgroundColor: "{colors.card}"
    rounded: "{rounded.lg}"
    padding: "20px"
  rail:
    backgroundColor: "{colors.cover}"
    textColor: "{colors.cover-ink}"
    width: "248px"
  rail-link-active:
    textColor: "{colors.card}"
    rounded: "{rounded.sm}"
    padding: "9px 10px"
  tag:
    backgroundColor: "{colors.ground-2}"
    textColor: "{colors.ink-soft}"
    typography: "{typography.label}"
    rounded: "{rounded.pill}"
    padding: "0 9px"
    height: "22px"
  tag-ok:
    backgroundColor: "{colors.ok-tint}"
    textColor: "{colors.ok}"
  tag-violated:
    backgroundColor: "{colors.stamp-tint}"
    textColor: "{colors.stamp}"
  tag-user:
    backgroundColor: "{colors.brass-tint}"
    textColor: "{colors.brass-ink}"
  verdict-stamp:
    textColor: "{colors.stamp}"
    typography: "{typography.stamp}"
    rounded: "{rounded.stamp}"
    padding: "6px 16px"
  dialog:
    backgroundColor: "{colors.card}"
    rounded: "{rounded.dialog}"
    width: "min(640px, calc(100vw - 32px))"
---

# Design System: Policy Service CMS

## Overview

**Creative North Star: "The Passbook"**

The console reads like a modern bank passbook rather than a generic admin template. A deep passbook-green cover rail holds the navigation; the working surface is a cool mint-tinted ground carrying white cards with hairline borders and soft green-tinted shadows. The signature is the dry-run statement: every applied policy prints as one ruled entry line in evaluation order, figures sit in tabular columns, a totals line closes each party's ledger under a heavy green rule, and a rotated rubber stamp lands with the verdict.

It is dense and operational, built for admins and QA who read numbers for a living. Color is sparing and semantic: green is the house and the primary action, stamp red only marks violations and destructive intent, brass marks the current place (active nav icon, scheduled state, overridden values, user-scoped records). The modern read comes from generous radii, layered soft depth, and short spring-like motion; the passbook read comes from ruled lines, numbered entries, and the stamp. The grey-sidebar-and-blue-accent admin template is the confirmed anti-reference.

**Key Characteristics:**
- Cover-green rail (248px) with a vertical green gradient; mint ground; white cards.
- Public Sans everywhere with tabular figures on by default.
- Ruled ledgers: 1px hairlines between rows, 2px cover-green rules for heads and totals.
- A fixed state-mark cell on every policy row.
- Stamp red reserved for violations and armed deletes; brass for "where you are" and "not yet".
- Short motion (120-420ms) on custom ease-out curves, fully disabled under reduced motion.

## Colors

A green house palette on a cool mint ground, with two seal colors (stamp red, brass) used as meaning rather than decoration.

### Primary
- **Passbook Cover Green** (cover): the rail, primary buttons, ledger rules (2px head and totals lines), the LIMIT and WAIVER tag ink, the outgoing-party avatar. `cover-2` is the primary hover and the rail's top gradient stop; `cover-deep` is the rail's bottom stop, the brand-logo glyph ink, and the raw JSON panel background.
- **Rail Ink** (cover-ink) and **Rail Muted** (cover-muted): text and group labels on the green rail only.

### Secondary
- **Ledger Brass** (brass): the active rail-link icon, brand logo tile, API-log count badge, and text selection. **Deep Brass** (brass-deep) draws the dashed scheduled ring and the overridden-input border; **Brass Tint** (brass-tint) with **Brass Ink** (brass-ink) fills FEE and user-scoped (Custom) tags and overridden inputs.

### Tertiary
- **Stamp Red** (stamp) with **Stamp Tint** (stamp-tint): violated entry rows, the REJECTED stamp, error notices, breach text, and the armed delete button. Never decorative.
- **Settled Green** (ok) with **Settled Tint** (ok-tint): the RESERVED stamp, success notices, "Within limits" and DISCOUNT tags.
- **Signal Green** (accent): links, focus rings, input focus border and 3px halo, caret, checkbox accent.

### Neutral
- **Mint Ground** (ground): page background. **Mint Ground 2** (ground-2): segmented-control track, default tag fill, skeleton base, inbound avatar.
- **Card White** (card): cards, inputs, dialogs. **Sunk** (sunk): row hover, ghost-button hover, dialog and statement footers, disabled inputs.
- **Hairline** (line) and **Hairline Strong** (line-strong): row and card borders; control borders and hovered-card borders.
- **Ink** (ink), **Ink Soft** (ink-soft), **Ink Faint** (ink-faint): body text, secondary copy and field labels, column heads and metadata.

### Named Rules
**The Seal Rule.** Stamp red means a rule was broken or something is about to be destroyed. It never appears as an accent, a brand color, or a chart hue.

**The Brass Means Here-or-Not-Yet Rule.** Brass marks the current location, scheduled state, a user override, or a user-scoped record. It is not a general highlight.

## Typography

**Display Font:** none; there is no display tier.
**Body Font:** Public Sans (with Segoe UI, system-ui)
**Label/Mono Font:** ui-monospace / Cascadia Mono / Consolas for raw JSON and the API log only.

**Character:** One humanist-grotesque family carries everything; hierarchy comes from weight (400, 600, 650, 700, 800) and small size steps, never from a second face.

### Hierarchy
- **Headline** (650, 22px, 1.25, -0.01em): page titles (h1), one per page.
- **Verdict** (650, 18px): the statement verdict line, colored stamp or ok.
- **Title** (650, 15px): card and dialog titles (h2), empty-state headings; h3 is 14px.
- **Body** (400, 14px, 1.5, tabular): all running text and table cells; intro copy capped at 70ch, empty-state copy at 52ch.
- **Label** (600, 12px): field labels, tags, sub-lines.
- **Column head** (600, 11px, 0.06em, uppercase): ledger and entry column heads, party titles (12px), rail group labels (0.08em).
- **Stamp** (800, 20px, 0.14em, uppercase; 16px on mobile): the verdict stamp only.

### Named Rules
**The Tabular Rule.** `font-variant-numeric: tabular-nums` is set on body; money and counts right-align in their own column so digits line up down the page.

## Layout

A two-column shell: a sticky full-height rail (248px) and a main column padded 28px top, 36px sides, 120px bottom (room for the floating API log), max 1480px. Pages open with a page head (title, 70ch intro, actions aligned to the bottom-right), then a toolbar (wrapping, 10px gap) and cards stacked at 18px. Tiles use an auto-fill grid of 250px minimum at 16px gaps; form rows auto-fit at 128px minimum with 12px gaps.

The dry-run page is a sticky transaction form (300px to 336px) beside a fluid statement card at a 24px gap; below 1180px it stacks. Below 780px the rail becomes a horizontal scrolling strip with a right-edge fade mask (brand, group labels and the whole rail foot hidden); the API log moves out of the rail into a separate floating cover-green button pinned bottom-left (12px inset, Lift shadow, brass count badge). Main padding drops to 20px/16px, page heads stack, and statement entries reflow into stacked label/value rows: the column heads hide, and each cell prints its `data-label` on the left and its value on the right.

Density is moderate-high: 11px/14px ledger cell padding, 46px entry rows, 36px buttons, 38px inputs.

## Elevation & Depth

Layered soft depth, always tinted with cover green (rgb 15 59 48), never neutral black. Surfaces rest at the lowest level; lift is a response to hover or to something floating above the page.

### Shadow Vocabulary
- **Rest** (`0 1px 2px rgb(15 59 48 / 0.06), 0 1px 1px rgb(15 59 48 / 0.04)`): cards, tiles, buttons, the selected segment.
- **Lift** (`0 2px 4px rgb(15 59 48 / 0.05), 0 8px 24px rgb(15 59 48 / 0.08)`): hovered link tiles (with a 3px rise), the floating mobile API-log button.
- **Float** (`0 12px 28px rgb(15 59 48 / 0.12), 0 32px 64px rgb(15 59 48 / 0.16)`): dialogs and the API log panel.
- **Primary press** (`0 1px 2px rgb(15 59 48 / 0.2), inset 0 1px 0 rgb(255 255 255 / 0.12)`): primary buttons only.

Dialog backdrops are cover-deep at 32% with a 3px blur.

### Named Rules
**The Green Shadow Rule.** Every shadow is cover green at low alpha. Depth deepens on hover; it is never a hard offset.

## Shapes

Gently rounded throughout: 12px for cards and tiles, 9px for the brand tile, 7px for buttons, inputs, notices, rail links and the segmented control, 14px for the API log, 16px for dialogs, full pills for tags and the count badge, circles for avatars, state marks and the swap button. Borders are 1px hairlines; ledger structure uses 2px cover-green rules. The stamp is the one tilted shape: an 8px-radius, 3px currentColor frame rotated -6deg.

## Components

### Buttons
Quiet, firm, and pressable.
- **Shape:** 7px radius, 36px tall, 14px side padding, 600 weight, 7px icon gap.
- **Default:** card white, 1px hairline-strong border, rest shadow; hover darkens the border to ink faint; active scales to 0.97.
- **Primary:** cover green, white text, primary-press shadow; hover to cover-2. One per form, at its foot.
- **Ghost:** transparent and borderless, 10px padding; hover fills sunk. Icon buttons are 32px square.
- **Danger (two-step):** a ghost "Delete" that turns stamp red on hover; first click arms a solid stamp-red "Delete X?" that pops in (scale 0.92 to 1, 200ms) and disarms after 4s.
- **Disabled:** 55% opacity, not-allowed cursor.

### Chips (tags)
- **Style:** 22px pills, 9px padding, 12px/600 text, no border.
- **Kinds:** LIMIT pale green on cover ink; WAIVER hollow with a cover-muted inset ring; FEE brass tint; DISCOUNT ok tint; TAX ground-2. Semantic: ok, violated, user (Custom).

### Cards / Containers
- **Corner Style:** 12px.
- **Background:** card white on mint ground.
- **Shadow Strategy:** Rest; link tiles rise 3px to Lift on hover.
- **Border:** 1px hairline; card heads separated by a hairline.
- **Internal Padding:** 20px body; 16px/20px heads; 18px tiles.

### Inputs / Fields
- **Style:** 38px tall, 11px padding, 7px radius, 1px hairline-strong border, card white; labels above at 12px/600 ink soft, hints below at 12px ink faint.
- **Focus:** border to signal green plus a 3px signal-green halo at 18%.
- **Disabled:** sunk fill, ink-faint text. **Overridden** (dry run): brass-deep border on brass tint.

### Navigation
- **Rail:** cover-green vertical gradient, 16px lucide icons at 1.75 stroke, 9px/10px link padding. Hover adds 7% white; current page 12% white, white text, brass icon. Group labels are 11px uppercase cover-muted. The rail foot holds the API-log button (6% white, brass count badge) and the API host line. Mobile: a horizontal scrolling strip, with the API log moved to a floating cover-green button bottom-left.

### Segmented control
A ground-2 track (3px padding) with transparent 30px buttons; the pressed segment becomes card white with ink text and the rest shadow.

### Ledger table
Uppercase 11px column heads over hairline rows, 11px/14px cells, sunk on hover, numbers right-aligned, muted rows in ink faint, actions column right-aligned.

### Passbook statement (signature)
The dry-run result card. It opens with a plain statement header, with no large figure and no gradient: the verdict line (18px, stamp or ok), then a breaches list naming each broken rule in stamp-red bold over ink-soft text. The header closes on a 2px cover rule, with the rotated stamp to its right. A bar beneath carries the evaluation time and the Statement/JSON segmented control. Each party (sender, receiver) gets an avatar (outgoing: cover with brass glyph; incoming: ground-2), a title, and a status tag. Its entries table uses a fixed colgroup: # (46px), policy (fluid), used before (104px, an editable right-aligned 30px input), after this (92px), rule (146px), charge (124px). Column heads sit on a 2px cover rule. Entry rows are 46px tall with 24px outer padding. Violated rows tint stamp. The totals line (fee, discount, tax, then the total charge) sits above a 2px cover rule in 700 weight. A sunk footer carries guidance and the secondary action. On mobile each entry becomes a stack of label/value rows.

### State marks (signature)
An 11px circle in a fixed cell on every policy row: filled cover green = tracked window, hollow cover ring = per-transaction, dashed brass-deep ring = scheduled, ink-faint ring struck diagonally = expired. A legend explains them.

### Notices, empty and loading
Notices are 7px-radius tinted bars (stamp or ok) with a 16px icon. Empty states center a 15px ink heading over 52ch soft copy. Loading uses shimmering 14px skeleton bars in table cells.

### Motion
Route content rises 10px over 320ms; children stagger 30ms apart up to 210ms; table rows only fade from 35% opacity (no transform on rows). Dialogs and the API log pop (14px rise, 0.98 scale, 260ms). The stamp lands from 1.7x scale over 420ms on `cubic-bezier(0.22, 1, 0.36, 1)`. Under `prefers-reduced-motion: reduce` all animation and transition is removed.

## Do's and Don'ts

### Do:
- **Do** use cover green for the rail, the single primary action per form, and ledger rules (2px heads and totals).
- **Do** keep stamp red for violations, errors and armed deletes only.
- **Do** give every policy row a state mark in a fixed cell, and keep the legend nearby.
- **Do** right-align money and counts in their own column; tabular figures are on by default.
- **Do** tint shadows cover green and lift link surfaces on hover (3px rise, Lift shadow).
- **Do** keep motion between 120ms and 420ms on the house curves and honor reduced motion.
- **Do** make destructive actions two-step: the first click arms and names what goes.

### Don't:
- **Don't** reintroduce the grey sidebar with a blue accent admin template.
- **Don't** use brass or stamp red as a decorative highlight.
- **Don't** use a neutral black shadow or a hard offset shadow.
- **Don't** add a second typeface; hierarchy comes from Public Sans weights and sizes.
- **Don't** animate transforms on table rows; rows fade only.
- **Don't** rotate anything but the verdict stamp.
