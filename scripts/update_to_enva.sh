#!/bin/bash
# Update all .smk files to use enva instead of conda run

set -e

echo "=========================================="
echo "Updating Snakemake rules to use enva..."
echo "=========================================="

# Directories to update
DIRS=(
    "inst/root_rules"
    "inst/rootless_rules"
)

# Counters
TOTAL=0
UPDATED=0
SKIPPED=0

for DIR in "${DIRS[@]}"; do
    if [ ! -d "$DIR" ]; then
        echo "⚠ Skipping non-existent directory: $DIR"
        continue
    fi

    echo ""
    echo "📁 Processing $DIR..."

    # Find all .smk files
    while IFS= read -r -d '' file; do
        TOTAL=$((TOTAL + 1))

        # Check if file contains "conda run -n"
        if grep -q "conda run -n" "$file"; then
            # Replace "conda run -n <env>" with "enva run <env>"
            # Example: "conda run -n otter-core" → "enva run otter-core"
            sed -i.bak 's/conda run -n \([^ ]\+\)/enva run \1/g' "$file"
            UPDATED=$((UPDATED + 1))
            echo "  ✓ Updated: $(basename "$file")"

            # Remove backup file
            rm -f "${file}.bak"
        else
            SKIPPED=$((SKIPPED + 1))
            if [ "$VERBOSE" = "1" ]; then
                echo "  - Skipped: $(basename "$file") (no conda run -n found)"
            fi
        fi
    done < <(find "$DIR" -name "*.smk" -print0)
done

echo ""
echo "=========================================="
echo "Summary:"
echo "  Total files scanned: $TOTAL"
echo "  Files updated:       $UPDATED"
echo "  Files skipped:       $SKIPPED"
echo "=========================================="

if [ $UPDATED -gt 0 ]; then
    echo ""
    echo "✓ Update complete!"
    echo ""
    echo "Next steps:"
    echo "  1. Review changes with: git diff"
    echo "  2. Test with: otter run --config otter.yaml --dry-run"
fi
