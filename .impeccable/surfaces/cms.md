---
version: 1
slug: "cms"
primary_target: "cms"
related_targets: []
---

# Policy Service CMS

Scope: the whole admin app in `cms/` (Next.js App Router). Visitor mode: Operate.
Audience: ops/product admins and developers/QA. Jobs: configure policy types, groups, system and custom policies; look up a wallet's effective config; reload cache; dry-run a transaction against live usage with optional overrides. Writes send `updatedBy: "admin"`.
Constraints: no auth yet; money as decimal strings; dry run saves nothing.

## Direction contract

THESIS: The Passbook. Every dry run prints as a bank passbook statement: one ruled entry line per applied policy in evaluation order, a running charge column, and a stamped verdict. Refuses the grey sidebar + blue accent admin template.
OWN-WORLD: Modern fintech, passbook-derived (user steer: "modern, not rigid old finance"). Deep passbook green #0f3b30 rail and primary actions; cool mint-tinted ground; white cards with 12px radii, hairline borders and soft layered shadows that deepen on hover; ink #15211d; stamp red #c2321f for violations and destructive actions; brass #e3b94f for active nav and scheduled state; emerald for RESERVED. Public Sans, tabular figures. Motion: route content rises in, list rows stagger, dialogs and the verdict stamp land with a short spring; 150-250ms, reduced-motion respected.
STORY: An admin edits a fee, presses Dry run, sees the entry line that produced each number and whether it would be RESERVED or REJECTED, overrides a usage total to test the edge, then goes back to edit.
FIRST VIEWPORT: Left cover-green rail (Dry run, Policy groups, Custom policies, Policy types, Effective config, Cache). Dry run page: transaction card left (wallet pair as two linked party cards); passbook statement card right with sender/receiver entries, totals line, rotated verdict stamp top-right. Primary action "Run dry run" at the foot of the form.
FORM: The Passbook, position 7 of my grounded list. Signature move: the passbook statement for dry-run results; state marks in a fixed cell (filled = tracked window, hollow = stateless, struck = expired, brass ring = scheduled) on every policy row. seed:cf34f999c55ba131
FINISH: unreviewed and undocumented is unfinished; this build ends with the finish review, the verdict, DESIGN.md, and every shipping raster carrying its provenance
