#!/usr/bin/env python3
"""
storage.py — JSON file-based persistence for the todo application.

Demonstrates @tag annotations for data access layer components.
"""

# @tag-block architecture
# @tag layer: "persistence"
# @tag pattern: "repository"
# @tag depends-on: ["models"]
import json
import os

from models import Task


# @tag-block data-domain
# @tag entity: "TaskStore"
# @tag description: "JSON file-backed repository for Task entities"
# @tag storage: "local-filesystem"
# @tag security: "file-permissions"
# @tag side-effects: ["reads-tasks-json", "writes-tasks-json"]
class TaskStore:
    """Persistent store for tasks, backed by a JSON file."""

    def __init__(self, path):
        # @tag-inline pii: "Store path may reveal filesystem layout"
        self.path = path
        self._tasks = {}
        self._next_id = 1
        self._load()

    # @tag-block api-endpoint
    # @tag action: "load"
    # @tag description: "Load tasks from JSON file on disk"
    # @tag side-effects: ["reads-filesystem"]
    # @tag error-states: ["file-not-found", "corrupt-json"]
    def _load(self):
        """Load tasks from the JSON file."""
        if not os.path.exists(self.path):
            self._tasks = {}
            self._next_id = 1
            return

        # @tag-inline critical-section: "JSON deserialization — may raise on corrupt data"
        try:
            with open(self.path, "r") as f:
                data = json.load(f)
        except (json.JSONDecodeError, OSError):
            # @tag-inline error-handling: "Corrupt store — resetting to empty"
            self._tasks = {}
            self._next_id = 1
            return

        # @tag-inline pii: "Task data may contain sensitive information in memory"
        tasks = data.get("tasks", [])
        self._tasks = {}
        for t in tasks:
            task = Task.from_dict(t)
            self._tasks[task.id] = task

        self._next_id = data.get("next_id", max(self._tasks.keys(), default=0) + 1)

    # @tag-block api-endpoint
    # @tag action: "flush"
    # @tag description: "Write all tasks to JSON file on disk"
    # @tag side-effects: ["writes-filesystem"]
    # @tag error-states: ["disk-full", "permission-denied"]
    def _flush(self):
        """Write all tasks to the JSON file."""
        # @tag-inline pii: "Task data persisted to disk — may contain personal information"
        data = {
            "next_id": self._next_id,
            "tasks": [t.to_dict() for t in self._tasks.values()],
        }
        # @tag-inline critical-section: "JSON serialization and file write"
        with open(self.path, "w") as f:
            json.dump(data, f, indent=2)

    # @tag-block api-endpoint
    # @tag action: "save"
    # @tag description: "Persist a new task (auto-assigns ID)"
    # @tag side-effects: ["writes-filesystem", "mutates-in-memory-state"]
    def save(self, task):
        """Save a new task, assigning it a unique ID."""
        task.id = self._next_id
        self._next_id += 1
        # @tag-inline audit: "Task created and stored with id %d" % task.id
        self._tasks[task.id] = task
        self._flush()

    # @tag-block api-endpoint
    # @tag action: "update"
    # @tag description: "Update an existing task in the store"
    # @tag side-effects: ["writes-filesystem"]
    def update(self, task):
        """Update an existing task."""
        if task.id in self._tasks:
            # @tag-inline audit: "Task %d updated — changes persisted" % task.id
            self._tasks[task.id] = task
            self._flush()

    # @tag-block api-endpoint
    # @tag action: "get"
    # @tag description: "Retrieve a single task by ID"
    # @tag side-effects: ["reads-memory"]
    def get(self, task_id):
        """Get a task by its ID."""
        return self._tasks.get(task_id)

    # @tag-block api-endpoint
    # @tag action: "list"
    # @tag description: "List all tasks (sorted by creation time)"
    # @tag side-effects: ["reads-memory"]
    def list(self):
        """List all tasks, sorted by creation time (oldest first)."""
        # @tag-inline pii: "Task titles exposed in-memory — avoid logging"
        return sorted(self._tasks.values(), key=lambda t: t.created_at)

    # @tag-block api-endpoint
    # @tag action: "delete"
    # @tag description: "Permanently remove a task by ID"
    # @tag side-effects: ["writes-filesystem", "destroys-data"]
    def delete(self, task_id):
        """Delete a task by its ID."""
        if task_id in self._tasks:
            # @tag-inline audit: "Task %d permanently deleted — data loss" % task_id
            del self._tasks[task_id]
            self._flush()
