#!/usr/bin/env python3
"""
Story lint script for Ora story files.

Exit codes:
  0 = clean
  1 = errors
  2 = warnings only
"""

import argparse
import re
from pathlib import Path

REQUIRED_FIELDS = ["Epic", "Status", "Priority", "Dependencies", "Target"]

SECTION_HEADERS = [
    "## 1. Objective",
    "## 2. User Story",
    "## 3. Scope",
    "## 4. Architecture Alignment",
    "## 5. Implementation Plan (Draft)",
    "## 6. Acceptance Criteria",
    "## 7. Verification Plan",
]

IMPLEMENTATION_SUBSECTIONS = [
    "### 5.1 Files to Create",
    "### 5.2 Files to Modify",
    "### 5.3 Tests to Add",
]

VERIFICATION_SUBSECTIONS = [
    "### Automated Tests",
    "### Manual Tests",
]

POST_IMPLEMENTATION_HEADERS = [
    "## Implementation Summary",
    "## Code Review Findings",
    "## Completion Status",
]

ID_PATTERN = re.compile(r"^[A-Z]{1,3}\.[0-9]{2}[A-Z]?$")


def extract_field(lines, field):
    pattern = re.compile(rf"^\*\*{re.escape(field)}:\*\*\s*(.+)$")
    for line in lines:
        match = pattern.match(line.strip())
        if match:
            return match.group(1).strip()
    return None


def extract_heading_id(lines):
    for line in lines:
        if line.startswith("# "):
            match = re.match(r"^#\s+([A-Z]{1,3}\.[0-9]{2}[A-Z]?)\s+-\s+(.+)$", line)
            if match:
                return match.group(1), match.group(2)
            return None, None
    return None, None


def extract_objective(lines):
    try:
        start = lines.index("## 1. Objective")
    except ValueError:
        return None
    for line in lines[start + 1:]:
        if line.startswith("## "):
            break
        if line.strip():
            return line.strip()
    return None


def parse_dependencies(value):
    if not value:
        return []
    if value.strip().lower() == "none":
        return []
    return re.findall(r"[A-Z]{1,3}\.[0-9]+[A-Z]?", value)


def lint_story(path, strict):
    errors = []
    warnings = []

    file_path = Path(path)
    if not file_path.exists():
        errors.append(f"File not found: {path}")
        return errors, warnings

    content = file_path.read_text(encoding="utf-8")
    lines = [line.rstrip("\n") for line in content.splitlines()]

    heading_id, heading_title = extract_heading_id(lines)
    if not heading_id or not heading_title:
        errors.append("Missing or invalid title line (# <ID> - <Title>).")
    else:
        file_id = file_path.stem.split("-")[0]
        if file_id != heading_id:
            errors.append(f"Title ID '{heading_id}' does not match filename ID '{file_id}'.")
        if not ID_PATTERN.match(heading_id):
            errors.append(f"Story ID '{heading_id}' must match pattern [A-Z].[0-9]{{2}}.")

    for field in REQUIRED_FIELDS:
        value = extract_field(lines, field)
        if not value:
            errors.append(f"Missing required field: {field}")

    status_value = extract_field(lines, "Status")
    if status_value and re.search(r"complete|implemented", status_value, re.IGNORECASE):
        warnings.append("Status indicates completion; new stories should not be Complete/Implemented.")

    deps_value = extract_field(lines, "Dependencies")
    deps = parse_dependencies(deps_value or "")
    if deps_value and deps_value.strip().lower() != "none" and not deps:
        warnings.append("Dependencies present but no parseable IDs found.")

    for header in SECTION_HEADERS:
        if header not in lines:
            errors.append(f"Missing section: {header}")

    for header in IMPLEMENTATION_SUBSECTIONS:
        if header not in lines:
            warnings.append(f"Missing implementation subsection: {header}")

    for header in VERIFICATION_SUBSECTIONS:
        if header not in lines:
            warnings.append(f"Missing verification subsection: {header}")

    if not any(line.strip().startswith("- [") for line in lines):
        errors.append("No checkbox items found (acceptance criteria or verification).")

    if "## 6. Acceptance Criteria" in lines:
        ac_index = lines.index("## 6. Acceptance Criteria")
        ac_lines = []
        for line in lines[ac_index + 1:]:
            if line.startswith("## "):
                break
            if line.strip().startswith("- ["):
                ac_lines.append(line)
        if not ac_lines:
            errors.append("Acceptance Criteria section has no checkbox items.")

    def strip_post_implementation_sections(lines):
        filtered = []
        skip = False
        for line in lines:
            if line in POST_IMPLEMENTATION_HEADERS:
                skip = True
                continue
            if skip and line.startswith("## "):
                skip = False
            if not skip:
                filtered.append(line)
        return "\n".join(filtered)

    def strip_code_blocks(text):
        """Remove fenced code blocks (``` ... ```) and inline code from text."""
        text = re.sub(r"```[\s\S]*?```", "", text)
        text = re.sub(r"`[^`]+`", "", text)
        return text

    filtered_content = strip_post_implementation_sections(lines)
    # Also strip code blocks before checking for placeholders
    content_without_code = strip_code_blocks(filtered_content)

    placeholder_matches = re.findall(r"<[^>]+>", content_without_code)
    if placeholder_matches:
        errors.append("Template placeholders (<...>) remain in the story.")

    if re.search(r"\bTBD\b|\bTODO\b", filtered_content, re.IGNORECASE):
        warnings.append("TBD/TODO placeholders found; resolve before implementation.")

    objective = extract_objective(lines)
    if not objective:
        warnings.append("Objective section appears empty.")

    if strict and warnings:
        errors.extend(warnings)
        warnings = []

    return errors, warnings


def main():
    parser = argparse.ArgumentParser(description="Lint Ora story files.")
    parser.add_argument("story_path", help="Path to story markdown file")
    parser.add_argument("--strict", action="store_true", help="Treat warnings as errors")
    args = parser.parse_args()

    errors, warnings = lint_story(args.story_path, args.strict)

    if errors:
        print("Errors:")
        for error in errors:
            print(f"  - {error}")
    if warnings:
        print("Warnings:")
        for warning in warnings:
            print(f"  - {warning}")

    if errors:
        raise SystemExit(1)
    if warnings:
        raise SystemExit(2)
    print("Story lint clean.")


if __name__ == "__main__":
    main()
