# Issue tracker: Local Markdown

Issues and specs live alongside existing development Markdown under `.scratch/`.
Publish to local files rather than GitHub Issues.

## Conventions

- One feature per directory: `.scratch/<feature-slug>/`.
- The spec is `.scratch/<feature-slug>/spec.md`.
- Implementation issues are separate files at
  `.scratch/<feature-slug>/issues/<NN>-<slug>.md`, numbered from `01`.
- Record triage state as a `Status:` line near the top, using
  `docs/agents/triage-labels.md`.
- Preserve existing task files and their recorded completion states.
- Append comments under a `## Comments` heading.
- Keep these files eligible for Git tracking, like existing development files.
  Commit only when the user requests it.

## Publish and fetch

To publish a spec or ticket, create its local Markdown file.
To fetch a ticket, read the referenced file.

## Wayfinding

- Map: `.scratch/<effort>/map.md`, containing Notes, Decisions-so-far, and Fog.
- Child tickets: `.scratch/<effort>/issues/<NN>-<slug>.md`.
- Record `Type:` as research, prototype, grilling, or task.
- Record dependencies in `Blocked by:`.
- Select the first numbered open, unblocked, unclaimed ticket.
- Claim with `Status: claimed` before working.
- Resolve by appending `## Answer`, setting `Status: resolved`,
  and adding an answer summary and file pointer to the map.
