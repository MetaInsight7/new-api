# Domain Docs

## Layout

This repo uses a single-context layout:
- Root `GLOSSARY.md` defines domain vocabulary.
- Root `docs/adr/` records architectural decisions.

## Before exploring

Read `GLOSSARY.md` and the ADRs relevant to the work.
If either is absent, proceed silently; create domain documents only when
the domain-modeling workflow resolves a term or decision.

## Consumer rules

Use glossary terms in specs, tickets, hypotheses, and test descriptions.
If a needed concept is missing, distinguish an actual domain gap from
an unnecessary synonym.

Surface conflicts with an accepted ADR explicitly rather than silently
overriding it.
