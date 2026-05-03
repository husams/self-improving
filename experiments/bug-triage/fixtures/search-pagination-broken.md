# Bug Report: Search results page 2+ shows duplicate results

**Reporter:** qa-team
**Reported:** 2026-05-02
**Affected service:** search-frontend (prod)
**First seen:** about 4 days ago after the v3.7 release

## Symptoms

- Pagination on `/search` is broken. Page 2 onwards shows the same items as
  page 1.
- This affects ~12% of searches (those where users go past page 1).
- Verified on Chrome 124, Firefox 125, Safari 17.4. Both desktop and mobile.

## Reproduction

1. Visit `/search?q=running+shoes`.
2. Scroll to bottom, click "Next page".
3. URL updates to `?q=running+shoes&page=2` but result list is unchanged.

## Logs

Frontend dev tools show the API call goes out:
```
GET /api/search?q=running+shoes&page=2 200 OK
```

But the response body has the same `results[]` as page 1. Backend team
confirmed the cursor parameter is being ignored due to a refactor in
commit `8f2e9a1`.

## Customer impact

Users can still find products via filters and category browsing.
Conversion rate on /search has dropped from 3.4% to 3.1% since the
release. Workaround: use category nav.

## Severity considerations

- Site is up, search is functional for page 1.
- Workaround exists.
- Affects discoverability for power users; ~$1.8K/day estimated lost revenue.
- Customer complaints: 4 in 4 days.
