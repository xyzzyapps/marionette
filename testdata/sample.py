# file: workers/cleanup.py

# @tag-block cron-job
# @tag schedule: "0 0 * * *"
# @tag data-domain: "analytics"
# @tag side-effects: ["purges-s3", "drops-cache"]
def nightly_purger():
    # @tag-inline security:pii-handling "Clears temporary sessions tracking IPs"
    session_store.clear_expired()
