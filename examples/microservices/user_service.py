#!/usr/bin/env python3
"""
user_service.py — A realistic microservice demonstrating Marionette annotation patterns.

This file showcases:
  - Architecture mapping with tag entry points
  - PII compliance and security auditing
  - Feature flag tracking
  - Agent directives (todos, ignore regions, skills)

Run with: python user_service.py
"""

# =============================================================================
# Architecture Mapping — Tag Entry Points & Microservice Boundaries
# =============================================================================

# @tag-block architecture
# @tag layer: "entrypoint"
# @tag protocol: "http"
# @tag domain: "user-management"
# @tag version: "v1"
# @tag team-owner: "platform-core"
# @tag depends-on: ["auth-service", "notification-service", "analytics-pipeline"]
# @tag discovered-by: "service-mesh-ingress"
import json
import os
import hashlib
import hmac
from datetime import datetime
from enum import Enum


# @tag-block architecture
# @tag layer: "domain-event"
# @tag event: "UserCreated"
# @tag producers: ["user-service"]
# @tag consumers: ["analytics-service", "email-service", "search-indexer"]
# @tag schema-version: "1.0"
class UserCreatedEvent:
    """Domain event emitted when a new user registers."""
    def __init__(self, user_id, email, name, signup_source):
        self.user_id = user_id
        self.email = email
        self.name = name
        self.signup_source = signup_source
        self.timestamp = datetime.utcnow().isoformat()
        self.event_type = "user.created"


# =============================================================================
# Compliance & Security Auditing — PII Tracking
# =============================================================================

# @tag-block api-endpoint
# @tag route: "POST /v1/users/register"
# @tag description: "Register a new user account"
# @tag layer: "controller"
# @tag security: "public"
# @tag security:pii-access
# @tag gdpr-impact: "high"
# @tag audit-category: "user-data-collection"
# @tag requires-review: "privacy-team"
def register_user(request_body):
    """
    Register a new user.
    
    @agent-skill "This handler collects PII (email, name, IP). Ensure GDPR consent is logged before storage."
    @agent-skill "After registration, dispatch UserCreatedEvent for downstream consumers."
    """
    # @tag-inline pii: "Email address — personal identifier, hash before logging"
    email = request_body.get("email")
    
    # @tag-inline pii: "Full name — personal data, encrypt at rest"
    name = request_body.get("name")
    
    # @tag-inline pii: "IP address — indirect identifier, GDPR Article 4(1)"
    ip_address = request_body.get("ip", request_metadata.get("client_ip"))
    
    # @tag-inline security:pii-access "Storing email + name — requires PII access log entry"
    user_id = store_user(email, name, ip_address)
    
    # @tag-inline audit: "User registration — GDPR consent recorded at timestamp"
    log_gdpr_consent(user_id, request_body.get("consent_version", "v1"))
    
    # @tag block feature-flag
    # @tag feature-flag: "onboarding-v2"
    # @tag description: "New onboarding flow with tutorial steps"
    # @tag owner: "product-growth"
    # @tag rollout: "10%"
    # @tag depends-on: ["tutorial-service"]
    # @agent-todo "Once onboarding-v2 hits 100%, remove the fallback path below and clean up feature flag checks"
    if feature_flags.is_enabled("onboarding-v2"):
        # @tag-inline feature-flag:hook "onboarding-v2 — tutorial step generator"
        schedule_onboarding_tutorial(user_id)
    
    # @tag-inline audit: "UserCreated event dispatched — consumers: analytics, email, search"
    dispatch(UserCreatedEvent(user_id, email, name, request_body.get("source", "web")))

    return {"user_id": user_id, "status": "created"}


# @tag-block api-endpoint
# @tag route: "GET /v1/users/{id}/profile"
# @tag description: "Retrieve user profile data"
# @tag layer: "controller"
# @tag security: "rbac"
# @tag security:pii-access
# @tag roles-required: ["user:self", "admin:any"]
# @tag gdpr-impact: "medium"
# @tag audit-category: "data-access"
def get_profile(user_id, requester_id):
    """
    Get user profile.
    
    @agent-skill "Profiles contain PII. Only return fields the requester is authorized for."
    @agent-skill "If requester is not the user or an admin, return 403."
    """
    # @tag security:pii-access "Read access to user profile — authorize before returning"
    authorize_or_403(requester_id, user_id)
    
    # @tag-inline pii: "Full user profile — includes email, phone, address"
    profile = load_user_profile(user_id)
    
    # @tag-inline audit: "Profile accessed by user %(requester_id)s — data-access audit trail"
    audit_log("profile.access", {"target": user_id, "requester": requester_id})
    
    return profile


# @tag-block api-endpoint
# @tag route: "DELETE /v1/users/{id}"
# @tag description: "Permanently delete user account and all associated data"
# @tag layer: "controller"
# @tag security: "admin-only"
# @tag security:pii-access
# @tag gdpr-impact: "critical"
# @tag audit-category: "data-deletion"
# @tag side-effects: ["purges-database", "clears-cache", "notifies-subprocessors"]
# @agent-todo "Add async GDPR deletion confirmation email after account purge"
def delete_user(user_id, requested_by):
    """
    Delete user — GDPR Right to Erasure (Article 17).
    
    @agent-skill "This deletes ALL user data. After deletion, notify all sub-processors."
    @agent-skill "Keep an anonymized deletion record for 30 days per GDPR Article 17(2)."
    """
    # @tag security:pii-access "Admin deleting user record — full PII purge"
    authorize_admin_or_403(requested_by)
    
    # @tag-inline audit: "GDPR deletion — user %(user_id)s erased by admin %(requested_by)s"
    audit_log("user.deletion", {"user_id": user_id, "requested_by": requested_by})
    
    # @tag-inline pii: "Anonymized deletion record — retains only hash and deletion date"
    store_deletion_record(user_id, requested_by)
    
    purge_user_data(user_id)
    invalidate_cache(user_id)
    notify_subprocessors(user_id)
    
    return {"status": "deleted", "user_id": user_id}


# =============================================================================
# Feature Flag Tracking — Experimental Features
# =============================================================================

# @tag-block feature-flag
# @tag feature-flag: "ai-recommendations"
# @tag description: "AI-powered product recommendations on user dashboard"
# @tag owner: "ml-team"
# @tag rollout: "5%"
# @tag tracking-issue: "PROD-2847"
# @tag targets: ["dashboard-page", "recommendation-engine"]
# @tag dependencies: ["ml-inference-service"]
# @agent-todo "When ai-recommendations graduates, remove flag guards and promote recommendation_engine to always-on"

# @tag-block feature-flag
# @tag feature-flag: "dark-mode"
# @tag description: "Dark mode UI theme toggle"
# @tag owner: "frontend-team"
# @tag rollout: "100%"
# @tag status: "permanent"
# @deprecated "This flag is always-on. Remove at next major cleanup."


# =============================================================================
# Agent Directives — Communicating with AI Agents via Comments
# =============================================================================

# @agent-todo "Implement rate limiting on all public endpoints. Use token bucket algorithm."
# @agent-todo "Add OpenTelemetry instrumentation to register_user() — missing tracing."
# @agent-todo "Extract PII fields into a @tag-block pii-schema block for automated compliance scanning."


# @agent-skill "When working on user_service.py, follow these conventions:"
# @agent-skill "  1. All PII access must be tagged with @tag security:pii-access"
# @agent-skill "  2. All new endpoints must have @tag-block api-endpoint with route and security metadata"
# @agent-skill "  3. Feature flags must reference a @tag-block feature-flag declaration"
# @agent-skill "  4. Domain events belong in a separate module, not in controllers"


# @agent-skill "Architecture context:"
# @agent-skill "  - user_service.py is part of the user-management domain"
# @agent-skill "  - It emits UserCreatedEvent consumed by 3 downstream services"
# @agent-skill "  - PII data flows to Postgres (encrypted) and Redis (cache, TTL 5min)"
# @agent-skill "  - The auth-service handles tokens; this service never sees raw passwords"


# ── @agent-ignore begin: Legacy V0 Compatibility Shim ──────────────────────
# Do not modify this block. It exists for backward compatibility with legacy
# mobile clients that have not yet migrated to the V1 API.
def legacy_register_v0(request_body):
    """DEPRECATED: Legacy V0 registration endpoint. Use register_user() instead."""
    # @tag-block deprecated
    # @tag deprecation: "v0-api-legacy"
    # @tag sunset-date: "2026-09-01"
    # @tag migration: "mobile-app-v2.1+"
    # @tag-inline pii: "Legacy endpoint — still handles PII, needs monitoring"
    return register_user(request_body)
# ── @agent-ignore end: Legacy V0 Compatibility Shim ────────────────────────


# =============================================================================
# Stubs — for demonstration purposes only
# =============================================================================
class feature_flags:
    _flags = {"onboarding-v2": True, "ai-recommendations": False, "dark-mode": True}
    @staticmethod
    def is_enabled(name):
        return feature_flags._flags.get(name, False)

request_metadata = {}
def store_user(email, name, ip): return 42
def log_gdpr_consent(uid, ver): pass
def schedule_onboarding_tutorial(uid): pass
def dispatch(event): pass
def authorize_or_403(r, u): pass
def load_user_profile(uid): return {"id": uid, "name": "Test"}
def audit_log(action, meta): pass
def authorize_admin_or_403(r): pass
def store_deletion_record(uid, r): pass
def purge_user_data(uid): pass
def invalidate_cache(uid): pass
def notify_subprocessors(uid): pass


if __name__ == "__main__":
    print("user_service.py — Marionette microservice example")
    print("\nArchitecture Mapping:")
    print("  - Entry point: POST /v1/users/register")
    print("  - Domain event: UserCreated → analytics, email, search")
    print("  - PII access points: 3 endpoints tagged with security:pii-access")
    print("  - Feature flags: onboarding-v2 (10%), ai-recommendations (5%), dark-mode (100%)")
    print("  - Agent directives: 6 todo items, 2 skill blocks, 1 ignore region")
    print("\nIndex with: marionette index examples/microservices")
    print("Query with: marionette query --key security:pii-access")
