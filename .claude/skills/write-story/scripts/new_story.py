#!/usr/bin/env python3
"""
Create a new Ora story from the template and update story indexes.
"""

import argparse
import re
from pathlib import Path

ROOT = Path(__file__).resolve().parents[4]
TEMPLATE_PATH = ROOT / ".claude" / "skills" / "write-story" / "references" / "story-template.md"

STATUS_MAP = {
    "complete": "\u2705 Complete",
    "implemented": "\u2705 Complete",
    "in progress": "\U0001F6A7 In Progress",
    "blocked": "\u26D4\uFE0F Blocked",
    "not started": "\U0001F6A7 To Do",
    "todo": "\U0001F6A7 To Do",
    "to do": "\U0001F6A7 To Do",
}

EPIC_MAP = {
    "F": {"name": "Foundations", "folder": "foundations", "heading": "### \U0001F3D7 Foundations (F)"},
    "A": {"name": "ASR Integration", "folder": "asr-integration", "heading": "### \U0001F399 ASR Integration (A)"},
    "L": {"name": "LLM Integration", "folder": "llm-integration", "heading": "### \U0001F9E0 LLM Integration (L)"},
    "T": {"name": "TTS Integration", "folder": "tts-integration", "heading": "### \U0001F5E3 TTS Integration (T)"},
    "X": {"name": "Tools", "folder": "tools", "heading": "### \U0001F6E0 Tools (X)"},
    "O": {"name": "Orchestration", "folder": "orchestration", "heading": "### \U0001F3BC Orchestration (O)"},
    "E": {"name": "Reliability", "folder": "reliability", "heading": None},
    "S": {"name": "Parakeet Starter", "folder": "parakeet-starter", "heading": None},
}


def slugify_title(title):
    slug = re.sub(r"[^A-Za-z0-9]+", "-", title.strip())
    slug = re.sub(r"-+", "-", slug).strip("-")
    return slug.upper()


def status_to_index_value(status):
    normalized = status.strip().lower()
    for key, value in STATUS_MAP.items():
        if key in normalized:
            return value
    return status


def build_story_content(template_text, replacements):
    content = template_text
    for key, value in replacements.items():
        content = content.replace(key, value)
    content = re.sub(r"<[^>]+>", "TBD", content)
    return content


def insert_or_update_row(lines, table_start, table_end, new_row, id_value, update_existing):
    existing_index = None
    for idx in range(table_start, table_end):
        if f"| **{id_value}** |" in lines[idx] or f"| {id_value} |" in lines[idx]:
            existing_index = idx
            break

    if existing_index is not None:
        if update_existing:
            lines[existing_index] = new_row
            return True, "updated"
        return False, "exists"

    rows = lines[table_start:table_end]
    insert_index = table_end
    for i, row in enumerate(rows, start=table_start):
        match = re.search(r"\|\s*\*\*([A-Z]\.[0-9]{2})\*\*\s*\|", row)
        if not match:
            match = re.search(r"\|\s*([A-Z]\.[0-9]{2})\s*\|", row)
        if match:
            current_id = match.group(1)
            if current_id > id_value:
                insert_index = i
                break
    lines.insert(insert_index, new_row)
    return True, "inserted"


def update_main_readme(epic_heading, row, update_existing):
    if not epic_heading:
        return False, "skipped"

    path = ROOT / "docs" / "stories" / "README.md"
    lines = path.read_text(encoding="utf-8").splitlines()

    try:
        heading_index = lines.index(epic_heading)
    except ValueError:
        return False, "missing heading"

    table_start = None
    table_end = None
    for i in range(heading_index + 1, len(lines)):
        if lines[i].startswith("|"):
            table_start = i + 2
            break
    if table_start is None:
        return False, "missing table"

    for i in range(table_start, len(lines)):
        if not lines[i].startswith("|"):
            table_end = i
            break
    if table_end is None:
        table_end = len(lines)

    updated, action = insert_or_update_row(
        lines,
        table_start,
        table_end,
        row,
        row.split("|")[1].strip(),
        update_existing=update_existing,
    )

    if updated:
        path.write_text("\n".join(lines) + "\n", encoding="utf-8")
    return updated, action


def update_epic_readme(epic_folder, row, story_id, update_existing):
    path = ROOT / "docs" / "stories" / epic_folder / "README.md"
    if not path.exists():
        return False, "missing epic README"

    lines = path.read_text(encoding="utf-8").splitlines()

    table_start = None
    table_end = None
    for i, line in enumerate(lines):
        if line.startswith("| Story "):
            table_start = i + 2
            break
    if table_start is None:
        return False, "missing table"

    for i in range(table_start, len(lines)):
        if not lines[i].startswith("|"):
            table_end = i
            break
    if table_end is None:
        table_end = len(lines)

    updated, action = insert_or_update_row(
        lines,
        table_start,
        table_end,
        row,
        story_id,
        update_existing=update_existing,
    )

    if updated:
        path.write_text("\n".join(lines) + "\n", encoding="utf-8")
    return updated, action


def main():
    parser = argparse.ArgumentParser(description="Create a new Ora story and update indexes.")
    parser.add_argument("--id", required=True, help="Story ID (e.g., F.10)")
    parser.add_argument("--title", required=True, help="Story title")
    parser.add_argument("--status", default="Not Started", help="Story status")
    parser.add_argument("--priority", default="P2 (Medium)", help="Priority label")
    parser.add_argument("--dependencies", default="None", help="Dependencies list or 'None'")
    parser.add_argument("--estimate", default="TBD", help="Estimated effort")
    parser.add_argument("--target", default="macOS 26 (Tahoe)", help="Target platform")
    parser.add_argument("--design", default="None", help="Design reference link or None")
    parser.add_argument("--objective", default="TBD", help="Objective text")
    parser.add_argument("--description", default=None, help="Epic index description override")
    parser.add_argument("--epic", default=None, help="Epic name override")
    parser.add_argument("--update-existing", action="store_true", help="Update existing index rows")
    parser.add_argument("--no-indexes", action="store_true", help="Skip index updates")
    parser.add_argument("--overwrite", action="store_true", help="Overwrite existing story file")
    parser.add_argument("--dry-run", action="store_true", help="Print paths without writing")
    args = parser.parse_args()

    if not TEMPLATE_PATH.exists():
        raise SystemExit(f"Template not found: {TEMPLATE_PATH}")

    if not re.match(r"^[A-Z]\.[0-9]{2}$", args.id):
        raise SystemExit("Story ID must match pattern [A-Z].[0-9]{2} (e.g., F.10)")

    prefix = args.id.split(".")[0]
    epic_info = EPIC_MAP.get(prefix)
    if not epic_info:
        raise SystemExit(f"Unsupported story prefix: {prefix}")

    epic_name = args.epic or epic_info["name"]
    epic_folder = epic_info["folder"]

    filename = f"{args.id}-{slugify_title(args.title)}.md"
    story_path = ROOT / "docs" / "stories" / epic_folder / filename

    if story_path.exists() and not args.overwrite:
        raise SystemExit(f"Story file already exists: {story_path}")

    replacements = {
        "<ID>": args.id,
        "<Title>": args.title,
        "<Epic>": epic_name,
        "<n days>": args.estimate,
        "<IDs (e.g., F.01, A.02) or None>": args.dependencies,
        "<link or None>": args.design,
        "<What problem this solves and why it matters.>": args.objective,
    }

    template_text = TEMPLATE_PATH.read_text(encoding="utf-8")
    content = build_story_content(template_text, replacements)
    content = re.sub(r"^\*\*Status\*\*:.+$", f"**Status:** {args.status}", content, flags=re.MULTILINE)
    content = re.sub(r"^\*\*Priority\*\*:.+$", f"**Priority:** {args.priority}", content, flags=re.MULTILINE)
    content = re.sub(r"^\*\*Estimated Effort\*\*:.+$", f"**Estimated Effort:** {args.estimate}", content, flags=re.MULTILINE)
    content = re.sub(r"^\*\*Dependencies\*\*:.+$", f"**Dependencies:** {args.dependencies}", content, flags=re.MULTILINE)
    content = re.sub(r"^\*\*Target\*\*:.+$", f"**Target:** {args.target}", content, flags=re.MULTILINE)
    content = re.sub(r"^\*\*Design Reference\*\*:.+$", f"**Design Reference:** {args.design}", content, flags=re.MULTILINE)

    if args.dry_run:
        print(f"Would write: {story_path}")
    else:
        story_path.write_text(content, encoding="utf-8")
        print(f"Created story: {story_path}")

    if args.no_indexes:
        return

    status_cell = status_to_index_value(args.status)

    main_row = f"| {args.id} | [{args.title}]({epic_folder}/{filename}) | {status_cell} |"
    updated_main, main_action = update_main_readme(
        epic_info["heading"],
        main_row,
        update_existing=args.update_existing,
    )

    if args.description:
        description = args.description
    else:
        description = args.objective if args.objective != "TBD" else "TBD"
    epic_row_parts = [
        f"| **{args.id}**",
        f"[{args.title}]({filename})",
        description,
        args.dependencies,
    ]

    epic_readme_path = ROOT / "docs" / "stories" / epic_folder / "README.md"
    epic_lines = []
    if epic_readme_path.exists():
        epic_lines = epic_readme_path.read_text(encoding="utf-8").splitlines()

    status_in_epic = any("| Story |" in line and "Status" in line for line in epic_lines)
    if status_in_epic:
        epic_row_parts.append(status_cell)

    epic_row = "| " + " | ".join(epic_row_parts) + " |"
    updated_epic, epic_action = update_epic_readme(
        epic_folder,
        epic_row,
        args.id,
        update_existing=args.update_existing,
    )

    print(f"Main README: {main_action}")
    print(f"Epic README: {epic_action}")


if __name__ == "__main__":
    main()
