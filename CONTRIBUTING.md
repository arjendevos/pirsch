# Contributing

Please open an issue to discuss the contribution you wish to make before submitting any changes. This way we can guide you through the process and give feedback.

## Syncing Your Branch with Main

When working on a feature or bugfix, you may need to sync your branch with the latest changes from the `main` branch without losing your own work. Here are two recommended approaches:

### Option 1: Rebase (Recommended for cleaner history)

Rebasing replays your commits on top of the latest main branch, creating a linear history:

```bash
# Save your current work (if you have uncommitted changes)
git add .
git commit -m "WIP: Save current work"

# Fetch the latest changes from the remote repository
git fetch origin

# Rebase your branch onto the latest main
git rebase origin/main

# If there are conflicts, resolve them and continue:
# 1. Fix conflicts in the affected files
# 2. Stage the resolved files: git add <file>
# 3. Continue the rebase: git rebase --continue

# Force push your rebased branch (only if you've already pushed it before)
git push --force-with-lease origin <your-branch-name>
```

### Option 2: Merge (Preserves all history)

Merging combines the histories of your branch and main, creating a merge commit:

```bash
# Save your current work (if you have uncommitted changes)
git add .
git commit -m "WIP: Save current work"

# Fetch the latest changes from the remote repository
git fetch origin

# Merge main into your branch
git merge origin/main

# If there are conflicts, resolve them:
# 1. Fix conflicts in the affected files
# 2. Stage the resolved files: git add <file>
# 3. Complete the merge: git commit

# Push your merged branch
git push origin <your-branch-name>
```

### Handling Uncommitted Changes

If you have uncommitted changes that you want to preserve:

```bash
# Stash your changes temporarily
git stash

# Update your branch (using either rebase or merge)
git fetch origin
git rebase origin/main  # or: git merge origin/main

# Restore your stashed changes
git stash pop

# If there are conflicts after popping the stash, resolve them manually
```

### Which Method Should I Use?

- **Use rebase** if you want a cleaner, linear commit history and are working on a feature branch that hasn't been shared with others extensively
- **Use merge** if you prefer to preserve the complete history of how branches evolved, or if multiple people are working on the same branch

## Licensing

This project is GNU AGPL licensed. If you make a contribution, you agree to transfer ownership of your contribution to us.
