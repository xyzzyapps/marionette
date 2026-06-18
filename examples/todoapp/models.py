#!/usr/bin/env python3
"""
models.py — Task data model with GTD-inspired fields and @tag annotations.

This module defines the core domain model for the todo application,
with extensive @tag annotations demonstrating Marionette's metadata extraction.
"""

# @tag-block data-domain
# @tag entity: "Task"
# @tag description: "Core domain model for a single actionable item"
# @tag lifecycle: ["created", "active", "completed", "archived"]
from enum import Enum
from datetime import datetime


# @tag-block data-domain
# @tag entity: "TaskStatus"
# @tag description: "Finite state machine for task lifecycle"
# @tag valid-transitions: ["active→done", "active→waiting", "waiting→active", "done→active"]
class TaskStatus(Enum):
    ACTIVE = "active"
    DONE = "done"
    WAITING = "waiting"


# @tag-block data-domain
# @tag entity: "Priority"
# @tag description: "Priority levels following Eisenhower Matrix principles"
# @tag mapping: {"critical": "urgent+important", "high": "important", "medium": "neither", "low": "delegate"}
class Priority(Enum):
    LOW = "low"
    MEDIUM = "medium"
    HIGH = "high"
    CRITICAL = "critical"


# @tag-block data-domain
# @tag entity: "Task"
# @tag fields: ["id", "title", "context", "priority", "status", "due", "project", "created", "updated"]
# @tag gtd-contexts: ["inbox", "work", "home", "errand", "phone", "computer", "anywhere"]
# @tag security: "no-auth"
class Task:
    """A single task item with GTD metadata fields."""

    def __init__(self, title, context="inbox", priority=Priority.MEDIUM,
                 due=None, project=None, status=TaskStatus.ACTIVE):
        # @tag-inline identity: "Auto-incrementing task ID — generated from store"
        self.id = None
        # @tag-inline pii: "Task title may contain user-identifying information"
        self.title = title
        self.context = context
        self.priority = priority if isinstance(priority, Priority) else Priority(priority)
        self.due = due
        self.project = project
        self.status = status if isinstance(status, TaskStatus) else TaskStatus(status)
        # @tag-inline timestamp: "Auto-generated creation and update timestamps"
        self.created_at = datetime.now().isoformat()
        self.updated_at = self.created_at

    # @tag-block api-endpoint
    # @tag action: "complete"
    # @tag description: "Transition task status to DONE"
    # @tag side-effects: ["mutates-status", "sets-completed-at"]
    # @tag security: "local-user"
    def complete(self):
        """Mark this task as completed."""
        # @tag-inline audit: "Status transition: active → done"
        self.status = TaskStatus.DONE
        self.updated_at = datetime.now().isoformat()
        # @tag-inline timestamp: "Record completion timestamp for analytics"
        self.completed_at = self.updated_at

    # @tag-block api-endpoint
    # @tag action: "wait"
    # @tag description: "Mark task as waiting on external dependency"
    # @tag side-effects: ["mutates-status"]
    # @tag security: "local-user"
    def wait(self):
        """Mark this task as waiting on something."""
        # @tag-inline audit: "Status transition: active → waiting"
        self.status = TaskStatus.WAITING
        self.updated_at = datetime.now().isoformat()

    # @tag-block api-endpoint
    # @tag action: "reactivate"
    # @tag description: "Move a done or waiting task back to active"
    # @tag side-effects: ["mutates-status"]
    # @tag security: "local-user"
    def reactivate(self):
        """Reactivate a task from done or waiting status."""
        # @tag-inline audit: "Status transition: any → active"
        self.status = TaskStatus.ACTIVE
        self.updated_at = datetime.now().isoformat()

    def to_dict(self):
        """Serialize task to dictionary for JSON storage."""
        return {
            "id": self.id,
            "title": self.title,
            "context": self.context,
            "priority": self.priority.value,
            "status": self.status.value,
            "due": self.due,
            "project": self.project,
            "created_at": self.created_at,
            "updated_at": self.updated_at,
            "completed_at": getattr(self, "completed_at", None),
        }

    @classmethod
    def from_dict(cls, data):
        """Deserialize task from dictionary."""
        task = cls(
            title=data["title"],
            context=data.get("context", "inbox"),
            priority=Priority(data.get("priority", "medium")),
            due=data.get("due"),
            project=data.get("project"),
            status=TaskStatus(data.get("status", "active")),
        )
        task.id = data.get("id")
        task.created_at = data.get("created_at", task.created_at)
        task.updated_at = data.get("updated_at", task.updated_at)
        if data.get("completed_at"):
            task.completed_at = data["completed_at"]
        return task

    def __repr__(self):
        return f"<Task #{self.id}: {self.title} [{self.context}]>"
