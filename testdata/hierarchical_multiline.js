// file: services/billing-engine.js
// Demonstrating hierarchical tagging and multiline strings with @tag annotations.

/**
 * @tag-block api-endpoint
 * @tag area/billing/tax-calculation: "v2"
 * @tag area/billing/invoicing: "v1"
 * @tag type: "backend"
 * @tag description: """
 *   Calculates tax for a given invoice line item.
 *   Supports VAT, GST, and sales tax regimes.
 *   Integrates with Avalara and TaxJar providers.
 * """
 * @tag compliance/regions: ["US", "EU", "APAC"]
 * @tag compliance/gdpr: "relevant"
 */
async function calculateTax(invoiceId, lineItem) {
    // @tag-inline area/billing/audit: "Tax calculation logged for compliance"
    const tax = await taxEngine.compute(lineItem.amount, lineItem.region);
    return { invoiceId, tax };
}

/**
 * @tag-block api-endpoint
 * @tag area/billing/invoicing: "v2"
 * @tag type: "backend"
 * @tag description: """
 *   Generates a PDF invoice for a completed order.
 *   Applies any pending credits and discounts.
 * """
 */
async function generateInvoice(orderId) {
    const invoice = await invoiceBuilder.build(orderId);
    return invoice;
}

// @tag-block cron-job
// @tag area/billing/reconciliation: "daily"
// @tag schedule: "0 2 * * *"
// @tag description: """
//   Daily reconciliation between billing system and payment processor.
//   Matches transactions, flags discrepancies, and sends alerts.
//   Owner: finance-ops team.
// """
function nightlyReconciliation() {
    // @tag-inline area/billing/audit: "Reconciliation run completed"
    reconcileAll();
}
