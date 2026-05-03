# Bug Report: Admin user table column header misaligned

**Reporter:** internal-tools-team
**Reported:** 2026-04-28
**Affected service:** admin-dashboard (internal only)

## Symptoms

The "Last login" column header in `/admin/users` is offset by 4 pixels to
the right of its column. The data underneath is correctly aligned. This is
purely cosmetic.

## Reproduction

1. Log into the admin dashboard.
2. Navigate to `/admin/users`.
3. Observe the "Last login" header — looks "off" compared to the others.

Visible on Chrome and Firefox. Safari 17 wraps the header so the issue is
hidden there.

## Customer impact

Internal tool only. Used by ~6 people on the support team. Nobody has
complained; flagged during a routine UI audit. No functional impact —
sorting, filtering, and reading the column all work correctly.

## Suggested fix

Likely a `padding-left` mismatch in `admin.css:1142`. Should be a one-line
change.
