# Bug Report: Payments completely failing in production

**Reporter:** ops-oncall
**Reported:** 2026-05-03 09:15 UTC
**Affected service:** payments-api (prod)

## Symptoms

- 100% of `POST /v1/charges` requests returning HTTP 502 since 09:08 UTC.
- Customer support ticket queue shows 47 tickets in the last 30 minutes from
  paying customers reporting their checkout failed.
- Stripe webhook delivery is queueing on Stripe's side; we are not
  processing any of them.
- Status page is currently green (not yet updated).

## Reproduction

1. Open any product page on shop.example.com.
2. Add to cart, proceed to checkout.
3. Submit any valid card.
4. Page hangs ~30s, then shows "Payment failed, please try again."

## Logs

```
2026-05-03T09:09:14Z payments-api ERROR upstream connection refused stripe-proxy:443
2026-05-03T09:09:14Z payments-api ERROR retry exhausted after 3 attempts
2026-05-03T09:09:14Z payments-api ERROR returning 502 to client
```

## Initial investigation

- `stripe-proxy` pod restarted 6 minutes ago and has been crashlooping with
  `panic: nil pointer in main.connectUpstream`.
- The bad code path was deployed in commit `a1b2c3d` 18 minutes ago.
- Revenue is currently $0/min vs the typical $4,200/min at this hour.

## Customer impact

All paying customers attempting checkout are blocked. We estimate ~$4,200
per minute in lost revenue. Multiple SLA-protected enterprise accounts
have already opened tickets.
