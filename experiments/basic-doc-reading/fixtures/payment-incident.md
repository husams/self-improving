# Payment Incident Report

Incident date: 2026-04-18

Timeline:

- 09:42 UTC: checkout latency increased after a payment worker deploy.
- 09:50 UTC: duplicate retry requests started appearing in the payment queue.
- 10:03 UTC: on-call paused the worker rollout.
- 10:17 UTC: queue depth returned to normal.
- 10:31 UTC: support confirmed that affected users could retry checkout.

Root cause:

The new payment worker reused the same idempotency key across unrelated retry batches.
This caused the gateway to reject some valid retries and delayed checkout completion.

Customer impact:

- 3.8% of checkout attempts between 09:42 and 10:17 UTC were delayed.
- No duplicate charges were found in the reconciliation job.
- 64 support tickets referenced checkout spinning or retry errors.

Follow-up work:

- Add a regression test for idempotency key generation.
- Add an alert when retry queue age exceeds five minutes.
- Review rollout checks for payment worker deploys.

