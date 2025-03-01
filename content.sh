#!/bin/bash

# Define the excluded patterns
EXCLUDE_PATTERN="./.git/|./vendor/|./uploads/|./downloads/|./output/|./reports/|./.idea/|./bin/|\.iml$|\.sum$|\.sh$|\.png$|.DS_Store|.gitignore|\.txt$|\.db$"

# Function to extract directory patterns
extract_folders() {
    # Use grep to find patterns that:
    # 1. Start with ./
    # 2. End with /
    # Then remove the ./ prefix and / suffix for tree command
    echo "$EXCLUDE_PATTERN" | grep -o '\./[^|]*/' | sed 's/^\.\///;s/\/$//'
}

# Function to generate tree exclude pattern
get_tree_pattern() {
    # Join the folders with | for tree command
    folders=$(extract_folders)
    echo "$folders" | paste -sd'|' -
}

# Usage message
usage() {
    echo "Usage: $0 [tree|content] [directory]"
    echo "  tree    : Display a tree view excluding specified patterns"
    echo "  content : Display the contents of files excluding specified patterns"
    echo "  directory: Optional. Path to the directory (defaults to current directory)"
    exit 1
}

# Check argument count
if [ "$#" -eq 0 ] || [ "$#" -gt 2 ]; then
    usage
fi

# Get command and directory
COMMAND="$1"
DIRECTORY="${2:-.}"  # Use second argument if provided, otherwise use current directory

# Validate directory
if [ ! -d "$DIRECTORY" ]; then
    echo "Error: Directory '$DIRECTORY' does not exist or is not a directory"
    exit 1
fi

# Check the command
if [ "$COMMAND" == "tree" ]; then
    tree_pattern=$(get_tree_pattern)
    echo "Generating tree output for '$DIRECTORY' (excluding: $tree_pattern)..."
    tree -I "$tree_pattern" "$DIRECTORY"
elif [ "$COMMAND" == "content" ]; then
    echo "Outputting file contents for '$DIRECTORY' (excluding specified patterns)..."
    find "$DIRECTORY" -type f | grep -vE "$EXCLUDE_PATTERN" | while read -r file; do
        echo "File: $file"
        cat "$file"
        echo "End of file: $file"
        echo "----------------------------------------"
    done
else
    usage
fi