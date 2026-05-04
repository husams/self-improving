---
name: csv-cleaner
description: Cleans and normalizes CSV files by stripping whitespace, deduplicating rows, and standardizing column names. Use when preparing CSV data for analysis, ingestion into databases, or when CSV files have inconsistent formatting.
---

# CSV Cleaner

## Overview

Normalizes CSV files for downstream consumption. Strips leading/trailing whitespace from cells, deduplicates exact-match rows, and converts column headers to snake_case.

## When to Use

Use this skill when:
- Preparing CSV exports for database ingestion
- Cleaning user-uploaded CSV files before processing
- Normalizing column names across multiple CSV sources

## Workflow

1. Run `scripts/clean.py <input.csv> <output.csv>` to clean a single file.
2. Verify the output row count matches expectations.
3. For batch operations, see [references/batch-mode.md](references/batch-mode.md).

## Notes

Trim only ASCII whitespace; preserve unicode whitespace inside quoted fields.
