#!/usr/bin/env python3
"""
todo.py — A GTD-style CLI task manager with @tag annotations for Marionette demo.

This example demonstrates how Marionette extracts structured @tag annotations
from a real application. Run it as:

    python todo.py add "Write proposal" --context @work --priority high
    python todo.py list
    python todo.py done 1
    python todo.py contexts
"""

# @tag-block architecture
# @tag layer: "presentation"
# @tag pattern: "cli-command"
# @tag depends-on: ["storage", "models"]
import argparse
import sys
import os

# @tag-inline pii: "Loads user task data from local filesystem"
sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))

from models import Task, TaskStatus, Priority
from storage import TaskStore


# @tag-block api-endpoint
# @tag command: "add"
# @tag description: "Create a new task with optional context, priority, and due date"
# @tag layer: "controller"
# @tag security: "local-user"
def cmd_add(args, store):
    """Add a new task."""
    # @tag-inline audit: "User creating new task record"
    task = Task(
        title=args.title,
        context=args.context or "inbox",
        priority=Priority(args.priority) if args.priority else Priority.MEDIUM,
        due=args.due,
        project=args.project,
    )
    store.save(task)
    # @tag-inline pii: "Task title may contain user-identifying information"
    print(f"  [ok] Created task {task.id}: {task.title}")


# @tag-block api-endpoint
# @tag command: "list"
# @tag description: "List tasks filtered by context, status, or priority"
# @tag layer: "controller"
# @tag security: "local-user"
def cmd_list(args, store):
    """List tasks with optional filters."""
    tasks = store.list()
    
    # @tag-inline filter: "Applies user-specified context filter"
    if args.context:
        tasks = [t for t in tasks if t.context == args.context]
    
    # @tag-inline filter: "Applies status filter"
    if args.status:
        tasks = [t for t in tasks if t.status.value == args.status]
    
    # @tag-inline filter: "Applies priority filter"
    if args.priority:
        tasks = [t for t in tasks if t.priority.value == args.priority]

    # @tag-inline audit: "Displaying filtered task list to user"
    if not tasks:
        print("  No tasks found.")
        return

    # @tag-block markdown-table
    # @tag purpose: "Render tasks as a readable table for terminal output"
    print(f"  {'ID':<4} {'Title':<40} {'Context':<12} {'Priority':<10} {'Status':<12}")
    print(f"  {'-'*4} {'-'*40} {'-'*12} {'-'*10} {'-'*12}")
    for t in tasks:
        print(f"  {t.id:<4} {t.title:<40} {t.context:<12} {t.priority.value:<10} {t.status.value:<12}")


# @tag-block api-endpoint
# @tag command: "done"
# @tag description: "Mark a task as completed by its ID"
# @tag layer: "controller"
# @tag security: "local-user"
# @tag side-effects: ["mutates-task-status"]
def cmd_done(args, store):
    """Mark a task as done."""
    # @tag-inline audit: "User completing task — status transition to DONE"
    task = store.get(args.task_id)
    if not task:
        print(f"  [x] Task {args.task_id} not found.")
        return
    task.complete()
    store.update(task)
    # @tag-inline analytics: "Task completion event — track velocity metrics"
    print(f"  [ok] Task {task.id} completed: {task.title}")


# @tag-block api-endpoint
# @tag command: "delete"
# @tag description: "Permanently remove a task by its ID"
# @tag layer: "controller"
# @tag security: "local-user"
# @tag side-effects: ["destroys-task-data"]
def cmd_delete(args, store):
    """Delete a task."""
    task = store.get(args.task_id)
    if not task:
        print(f"  [x] Task {args.task_id} not found.")
        return
    title = task.title
    store.delete(args.task_id)
    # @tag-inline audit: "User permanently deleting task — data loss event"
    print(f"  [ok] Deleted task {args.task_id}: {title}")


# @tag-block api-endpoint
# @tag command: "contexts"
# @tag description: "Show all tasks grouped by GTD context"
# @tag layer: "controller"
# @tag security: "local-user"
def cmd_contexts(args, store):
    """Show tasks grouped by context."""
    tasks = store.list()
    contexts = {}
    for t in tasks:
        contexts.setdefault(t.context, []).append(t)
    
    for ctx in sorted(contexts.keys()):
        print(f"\n  @{ctx}")
        for t in contexts[ctx]:
            print(f"    [{t.id}] {t.title} — {t.priority.value}")


# @tag-block data-domain
# @tag entity: "task"
# @tag fields: ["id", "title", "context", "priority", "status", "due", "project", "created"]
def build_parser():
    """Build the CLI argument parser."""
    parser = argparse.ArgumentParser(
        description="todo — A GTD-style task manager (Marionette demo)",
        epilog="Use @tag annotations for structured metadata extraction.",
    )
    parser.add_argument("--store", default=None, help="Path to task store file")
    
    sub = parser.add_subparsers(dest="command")
    
    # add
    p_add = sub.add_parser("add", help="Add a new task")
    p_add.add_argument("title", help="Task title")
    p_add.add_argument("--context", "-c", help="GTD context (@work, @home, etc.)")
    p_add.add_argument("--priority", "-p", choices=["low", "medium", "high", "critical"], help="Priority level")
    p_add.add_argument("--due", "-d", help="Due date (YYYY-MM-DD)")
    p_add.add_argument("--project", help="Project name")
    
    # list
    p_list = sub.add_parser("list", help="List tasks")
    p_list.add_argument("--context", "-c", help="Filter by context")
    p_list.add_argument("--status", "-s", choices=["active", "done", "waiting"], help="Filter by status")
    p_list.add_argument("--priority", "-p", choices=["low", "medium", "high", "critical"], help="Filter by priority")
    
    # done
    p_done = sub.add_parser("done", help="Mark task as done")
    p_done.add_argument("task_id", type=int, help="Task ID")
    
    # delete
    p_delete = sub.add_parser("delete", help="Delete a task")
    p_delete.add_argument("task_id", type=int, help="Task ID")
    
    # contexts
    sub.add_parser("contexts", help="Show tasks by context")
    
    return parser


# @tag-block api-endpoint
# @tag command: "entrypoint"
# @tag description: "Main entry point — dispatches CLI commands"
# @tag layer: "controller"
# @tag security: "local-user"
# @tag side-effects: ["reads-writes-task-store"]
def main():
    # @tag-inline init: "Bootstrap application — load parser and task store"
    parser = build_parser()
    args = parser.parse_args()
    
    if not args.command:
        parser.print_help()
        return
    
    store_path = args.store or os.path.join(
        os.path.dirname(os.path.abspath(__file__)), "tasks.json"
    )
    
    # @tag-inline pii: "Task store path may reveal user's home directory structure"
    store = TaskStore(store_path)
    
    commands = {
        "add": cmd_add,
        "list": cmd_list,
        "done": cmd_done,
        "delete": cmd_delete,
        "contexts": cmd_contexts,
    }
    
    # @tag-inline dispatch: "Command dispatch — routes parsed arguments to handler"
    fn = commands.get(args.command)
    if fn:
        fn(args, store)
    else:
        parser.print_help()


if __name__ == "__main__":
    main()
