#!/usr/bin/env python3
"""
payment_service.py — Tag template expansion demo for Marionette.

This file demonstrates @tag-use template expansion.
Templates are defined in .marionetterc.json and expanded inline during indexing.

Usage:
    marionette --db demo.db index examples/tag-templates
    marionette --db demo.db query --key area/billing
"""


# @tag-use billing-endpoint
# @tag route: "POST /v1/payments/charge"
# @tag idempotency: "idempotency-key"
def charge_payment(card_token, amount_cents):
    """Charge a payment using the billing template."""
    # @tag-inline audit: "Payment charged — idempotency key logged"
    result = payment_gateway.charge(card_token, amount_cents)
    return {"status": "charged", "amount": amount_cents}


# @tag-use billing-endpoint
# @tag route: "POST /v1/payments/refund"
# @tag side-effects: ["reverses-charge", "sends-notification"]
# @tag requires-review: "finance-team"
def refund_payment(charge_id, reason):
    """Refund a payment — all billing-endpoint tags auto-applied."""
    # @tag-inline audit: "Refund initiated — finance team notified"
    result = payment_gateway.refund(charge_id, reason)
    return {"status": "refunded", "charge_id": charge_id}


# @tag-use public-api-endpoint
# @tag route: "GET /v1/payments/methods"
# @tag cache: "ttl=300"
def list_payment_methods(customer_id):
    """List available payment methods — public endpoint."""
    return payment_gateway.methods(customer_id)


# @tag-use cron-job
# @tag schedule: "0 3 * * *"
# @tag description: "Daily reconciliation of unsettled transactions"
def reconcile_payments():
    """Daily reconciliation — cron-job template auto-applies monitoring tags."""
    unsettled = payment_gateway.get_unsettled()
    for tx in unsettled:
        reconcile(tx)
    # @tag-inline audit: "Daily reconciliation complete — %d transactions" % len(unsettled)
    log_audit("reconciliation", {"count": len(unsettled)})
