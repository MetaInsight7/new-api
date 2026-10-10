# Console Defaults and Request IP Logging

## Status

Accepted

## Context

The TokenDazi console is moving closer to the TokenDazi overlay behavior. The
remaining differences are the default desktop layout, the available-models
description, request IP recording, and workspace presentation. The current
base branch stores request and error IPs only when a per-user setting is
enabled, while the overlay records them for every request.

The available-models frontend contains a compatibility branch for a
`fixed_account` response, but the current backend resolver only returns
`selectable_group`. The fixed-account branch must remain harmless for future
responses without keeping the group-specific description visible in the
normal current view.

## Decisions

1. Use the `sidebar` layout variant as the default for users without a
   `layout_variant` cookie. Existing layout cookies continue to win, and the
   existing collapse behavior is unchanged.
2. Remove only the selectable-group description from available-models. Keep
   the fixed-account description as a defensive fallback for a future backend
   response.
3. Always record the client IP for consume and error logs using the existing
   Gin `ClientIP()` trusted-proxy behavior. Remove the per-user IP-recording
   setting and its profile/security controls. Legacy JSON settings containing
   `record_ip_log` are ignored during decoding.
4. Keep `Current Balance` static. Consumption, request, and token summary
   titles continue to reflect the selected time range, including custom
   ranges, as in the overlay.
5. Align the workspace right column at desktop widths by using a fixed
   `24rem` column for both the announcement card and the integration card.
   Smaller viewports continue to stack responsively.

## Implementation tickets

- Change the frontend layout provider default variant to `sidebar`.
- Remove the selectable-group available-models description and its active
  translations.
- Make consume/error log IP assignment unconditional and remove the legacy
  user-setting field from backend and frontend contracts.
- Align workspace grid columns without changing the API or card content.
- Verify changed frontend behavior with focused tests and Playwright.

## Acceptance criteria

- A fresh browser with no `layout_variant` cookie renders the standard sidebar;
  an existing explicit layout cookie is preserved.
- The selectable-group available-models page does not render the removed
  sentence, while the fixed-account fallback remains available in code.
- A consume or error log created through a request context stores its client
  IP without reading a user preference.
- No profile/security UI or update payload exposes `record_ip_log`.
- Workspace announcement and integration cards have the same desktop width.
- Workspace summary keeps the balance title stable and changes the other three
  period labels with the selected range.
