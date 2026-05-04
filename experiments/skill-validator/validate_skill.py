#!/usr/bin/env python3
"""
Skill validator - checks SKILL.md files against authoring best practices.
"""

import sys
import re
from pathlib import Path
from typing import List, Tuple

# Exit codes
EXIT_SUCCESS = 0
EXIT_FAILURE = 1

# ANSI colors
RED = '\033[91m'
YELLOW = '\033[93m'
GREEN = '\033[92m'
RESET = '\033[0m'


class ValidationResult:
    def __init__(self):
        self.errors: List[str] = []
        self.warnings: List[str] = []

    def add_error(self, message: str):
        self.errors.append(f"{RED}❌ {message}{RESET}")

    def add_warning(self, message: str):
        self.warnings.append(f"{YELLOW}⚠️  {message}{RESET}")

    def has_errors(self) -> bool:
        return len(self.errors) > 0

    def print_results(self):
        if self.errors:
            print("\nCritical Issues (must fix):")
            for error in self.errors:
                print(f"  {error}")

        if self.warnings:
            print("\nWarnings (recommended to address):")
            for warning in self.warnings:
                print(f"  {warning}")

        if not self.errors and not self.warnings:
            print(f"\n{GREEN}✅ All validation checks passed!{RESET}")


def parse_frontmatter(content: str) -> Tuple[dict, str]:
    """Extract YAML frontmatter and body from SKILL.md content."""
    if not content.startswith('---\n'):
        return {}, content

    parts = content.split('---\n', 2)
    if len(parts) < 3:
        return {}, content

    frontmatter_text = parts[1]
    body = parts[2]

    # Simple YAML parser for name and description
    frontmatter = {}
    for line in frontmatter_text.strip().split('\n'):
        if ':' in line:
            key, value = line.split(':', 1)
            frontmatter[key.strip()] = value.strip()

    return frontmatter, body


def validate_skill(skill_path: Path) -> ValidationResult:
    """Validate a skill directory."""
    result = ValidationResult()

    # Check SKILL.md exists
    skill_md = skill_path / "SKILL.md"
    if not skill_md.exists():
        result.add_error(f"SKILL.md not found in {skill_path}")
        return result

    # Read content
    content = skill_md.read_text()
    frontmatter, body = parse_frontmatter(content)

    # Validate frontmatter
    if not frontmatter:
        result.add_error("Missing YAML frontmatter")
        return result

    # Check required fields
    if 'name' not in frontmatter:
        result.add_error("Missing 'name' field in frontmatter")
    else:
        validate_name(frontmatter['name'], result)

    if 'description' not in frontmatter:
        result.add_error("Missing 'description' field in frontmatter")
    else:
        validate_description(frontmatter['description'], result)

    # Check for XML tags in frontmatter
    frontmatter_text = content.split('---\n', 2)[1] if '---\n' in content else ""
    if '<' in frontmatter_text and '>' in frontmatter_text:
        result.add_error("XML/HTML tags found in frontmatter")

    # Validate body content
    validate_body(body, result)

    # Validate file structure
    validate_structure(skill_path, body, result)

    return result


def validate_name(name: str, result: ValidationResult):
    """Validate skill name format."""
    # Check length
    if len(name) > 64:
        result.add_error(f"Skill name exceeds 64 characters ({len(name)} chars)")

    # Check format: lowercase, hyphens only
    if not re.match(r'^[a-z0-9]+(-[a-z0-9]+)*$', name):
        result.add_error(f"Skill name '{name}' must be lowercase with hyphens only (no underscores, spaces, or special chars)")

    # Check for reserved words
    reserved_words = ['anthropic', 'claude']
    for word in reserved_words:
        if word in name:
            result.add_error(f"Skill name contains reserved word '{word}'")


def validate_description(desc: str, result: ValidationResult):
    """Validate description quality."""
    # Check length
    if len(desc) < 50:
        result.add_warning(f"Description is short ({len(desc)} chars). Aim for at least 50 characters with specific details.")

    # Check for third-person perspective
    first_second_person = re.search(r'\b(you|your|I|we|our|my)\b', desc, re.IGNORECASE)
    if first_second_person:
        result.add_error(f"Description uses first/second person ('{first_second_person.group()}'). Use third person (e.g., 'Validates...', 'Creates...')")

    # Check for when-to-use triggers
    has_trigger = re.search(r'\b(use when|for|when)\b', desc, re.IGNORECASE)
    if not has_trigger:
        result.add_warning("Description missing when-to-use triggers (e.g., 'Use when...', 'For...')")

    # Check for TODO placeholders
    if 'TODO' in desc.upper():
        result.add_error("Description contains TODO placeholder")


def validate_body(body: str, result: ValidationResult):
    """Validate SKILL.md body content."""
    lines = body.split('\n')

    # Check line count
    if len(lines) > 500:
        result.add_warning(f"SKILL.md body is long ({len(lines)} lines). Consider moving detailed content to references/")

    # Check for TODO placeholders
    if 'TODO' in body.upper():
        result.add_error("SKILL.md contains TODO placeholder(s)")

    # Check for Windows-style paths
    if re.search(r'[A-Z]:\\', body):
        result.add_error("Windows-style paths detected (use forward slashes)")

    # Check for first/second person (more lenient in body, warn only)
    inconsistent_person = re.search(r'^[^`\n]*\b(you should|you must|you can)\b', body, re.IGNORECASE | re.MULTILINE)
    if inconsistent_person:
        result.add_warning("Found second person in instructions. Use imperative form (e.g., 'Run...' instead of 'You should run...')")


def validate_structure(skill_path: Path, body: str, result: ValidationResult):
    """Validate file structure and references."""
    # Check for file references in body
    file_refs = re.findall(r'\[([^\]]+)\]\(([^)]+)\)', body)

    for ref_text, ref_path in file_refs:
        # Skip external URLs
        if ref_path.startswith('http://') or ref_path.startswith('https://'):
            continue

        # Check depth (should be one level: references/file.md or scripts/file.py)
        path_parts = ref_path.split('/')
        if len(path_parts) > 2:
            result.add_warning(f"File reference '{ref_path}' is deeply nested. Keep references one level deep from SKILL.md")

        # Check if file exists
        full_path = skill_path / ref_path
        if not full_path.exists():
            result.add_error(f"Referenced file not found: {ref_path}")

    # Check scripts/ directory if it exists
    scripts_dir = skill_path / "scripts"
    if scripts_dir.exists() and scripts_dir.is_dir():
        scripts = list(scripts_dir.glob('*'))
        if not scripts:
            result.add_warning("Empty scripts/ directory found")


def main():
    if len(sys.argv) != 2:
        print("Usage: validate_skill.py <path-to-skill-directory>")
        sys.exit(EXIT_FAILURE)

    skill_path = Path(sys.argv[1])

    if not skill_path.exists():
        print(f"Error: Skill path does not exist: {skill_path}")
        sys.exit(EXIT_FAILURE)

    if not skill_path.is_dir():
        print(f"Error: Skill path is not a directory: {skill_path}")
        sys.exit(EXIT_FAILURE)

    print(f"Validating skill: {skill_path}")
    print("-" * 60)

    result = validate_skill(skill_path)
    result.print_results()

    if result.has_errors():
        sys.exit(EXIT_FAILURE)
    else:
        sys.exit(EXIT_SUCCESS)


if __name__ == '__main__':
    main()
