# Batch Mode

## Overview

Process multiple CSV files in a single operation using the batch cleaning script.

## Usage

```bash
scripts/clean_batch.py <input-directory> <output-directory>
```

**Example:**
```bash
scripts/clean_batch.py data/raw/ data/cleaned/
```

## Options

- `--parallel` - Process files concurrently (default: sequential)
- `--skip-errors` - Continue processing if individual files fail

## Output

Preserves directory structure from input to output directory. Failed files are logged to `batch-errors.log`.
