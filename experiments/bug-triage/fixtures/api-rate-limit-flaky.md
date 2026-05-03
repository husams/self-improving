# Bug Report: API rate-limit headers occasionally missing

**Reporter:** partner-engineering
**Reported:** 2026-05-01
**Affected service:** public-api (prod)

## Symptoms

Several enterprise API partners report that approximately 1 in 50 responses
from `/v2/*` endpoints is missing the `X-RateLimit-Remaining` header. When
the header is missing, partners cannot back off proactively and a few have
hit hard 429s as a result.

## Reproduction

Hard to reproduce on demand. We've observed it in production logs by
sampling 10,000 responses: 198 were missing the header (1.98%). Pattern
suggests it correlates with the `traefik` ingress restarting one of its
replicas mid-request, but we have not confirmed.

## Logs

When the header is missing, the access log shows the request was served
by the API but the response went out without the rate-limit middleware
being invoked. No error logs are produced.

## Customer impact

- 3 enterprise partners have raised support tickets in the last week.
- Two of them hit 429s during peak traffic and had to back off manually,
  which their integration handled but with a noticeable spike in latency
  on their dashboards.
- One partner explicitly asked for an ETA on a fix, citing it makes it
  hard to honor their own SLAs to downstream users.
- API itself remains functional; correct rate-limit accounting is still
  enforced server-side, so partners are not over-limited — they just lose
  visibility 2% of the time.

## Workarounds

Partners can call `/v2/quota` to check remaining limits; not realtime but
usable. Documented in the developer portal already.
