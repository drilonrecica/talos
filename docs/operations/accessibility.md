# Accessibility

Binnacle targets WCAG 2.2 Level AA. Automated checks run with `@axe-core/playwright` against the login, setup, onboarding, overview, server, resources, resource detail, events, settings, and monitor-health pages.

## Automated coverage

Run the accessibility smoke suite:

```bash
pnpm --dir web test:e2e a11y.spec.ts
```

The tests scan each page with `wcag2a`, `wcag2aa`, `wcag21a`, and `wcag21aa` rules and fail on any violation.

## Manual checklist for releases

Verify each item in a supported browser with keyboard-only interaction and, where available, a screen reader.

### Page structure

- [ ] Skip-to-content link is the first focusable element and moves focus to `#content`.
- [ ] Each page has exactly one `<main>` landmark.
- [ ] Page `<title>` is "Binnacle" and route-specific context is conveyed by the first heading.
- [ ] Primary navigation uses `<nav aria-label="Primary navigation">` and marks the current page with `aria-current="page"`.

### Login and setup

- [ ] Username and password fields have visible, programmatic labels.
- [ ] Focus moves to the error message after a failed submission.
- [ ] Password fields use `type="password"` and appropriate autocomplete tokens.

### Color and visual information

- [ ] Status badges (healthy, degraded, down, unknown, archived) are readable with the text label; the colored dot is `aria-hidden`.
- [ ] Warning text meets 4.5:1 contrast on both light and dark themes.
- [ ] Charts are accompanied by a visible text summary and a keyboard-focusable inspector button.

### Interaction

- [ ] All buttons and links show a visible focus indicator.
- [ ] Dialogs trap focus while open and return focus to the triggering element on close.
- [ ] Disclosure widgets (details/summary) and tab-like navigation are keyboard operable.
- [ ] Forms can be completed and submitted using only the keyboard.

### Motion

- [ ] Enable `prefers-reduced-motion` and confirm transitions and animations are suppressed.
- [ ] Live updates do not auto-scroll or flash more than three times per second.

### Live regions

- [ ] Loading states use `role="status"`.
- [ ] Errors use `role="alert"`.
- [ ] Connection and deletion progress are announced politely.
