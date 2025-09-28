#!/bin/bash

read -p "Enter full git repository URL: " NEW_URL

if [ ! -d ".git" ]; then
    echo "Error: This directory is not a git repository."
    exit 1
fi

git remote remove origin 2>/dev/null
git remote add origin "$NEW_URL"
CURRENT_BRANCH=$(git branch --show-current)
git branch --set-upstream-to=origin/"$CURRENT_BRANCH" "$CURRENT_BRANCH" 2>/dev/null || \
echo "Git origin updated to: $NEW_URL"