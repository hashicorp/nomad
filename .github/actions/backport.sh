#!/bin/bash
set -ex

# Backport script for cherry-picking merged PRs to release branches
# This script auto-detects squash-merge vs regular merge commits
# Works with any label prefixed with 'release/' - the label name becomes the target branch

# Required environment variables:
# - GITHUB_TOKEN: GitHub token for gh CLI
# - PR_NUMBER: Original PR number
# - PR_TITLE: Original PR title
# - COMMIT_SHA: The commit SHA to cherry-pick
# - TARGET_BRANCH: Target branch for backport (extracted from release/* label)
# - GITHUB_REPOSITORY: Repository in owner/repo format

echo "PR: #${PR_NUMBER}"
echo "Commit: ${COMMIT_SHA}"
echo "Target branch: ${TARGET_BRANCH}"

git config user.name "github-actions[bot]"
git config user.email "github-actions[bot]@users.noreply.github.com"

# Verify target branch exists
if ! git ls-remote --exit-code --heads origin "${TARGET_BRANCH}" > /dev/null 2>&1; then
    echo "✗ Target branch '${TARGET_BRANCH}' does not exist"
    exit 1
fi

git fetch origin "${TARGET_BRANCH}"
git fetch origin main

BACKPORT_BRANCH="backport-${PR_NUMBER}-to-${TARGET_BRANCH//\//-}"
git checkout -b "${BACKPORT_BRANCH}" "origin/${TARGET_BRANCH}"

# Auto-detect merge type
echo "Detecting commit type..."
PARENT_COUNT=$(git rev-list --parents -n 1 "${COMMIT_SHA}" | wc -w)
PARENT_COUNT=$((PARENT_COUNT - 1))  # Subtract 1 to get actual parent count

echo "Commit has ${PARENT_COUNT} parent(s)"

# Perform cherry-pick based on commit type
if [ "${PARENT_COUNT}" -eq 1 ]; then
    echo "Detected squash-merge commit, using simple cherry-pick"
    CHERRY_PICK_CMD="git cherry-pick ${COMMIT_SHA}"
else
    echo "Detected merge commit, using cherry-pick with -m 1"
    CHERRY_PICK_CMD="git cherry-pick -m 1 ${COMMIT_SHA}"
fi

echo "Executing: ${CHERRY_PICK_CMD}"

# Attempt cherry-pick
if ${CHERRY_PICK_CMD}; then
    echo "✓ Cherry-pick successful"

    COMMIT_MESSAGE=$(git log -1 --pretty=format:"%B" HEAD)

    echo "Pushing backport branch..."
    git push origin "${BACKPORT_BRANCH}"

    BODY=$(cat <<EOF
Backport of https://github.com/${GITHUB_REPOSITORY}/pull/${PR_NUMBER}

---

${COMMIT_MESSAGE}
EOF
)

    echo "Creating backport PR..."
    gh pr create \
        --base "${TARGET_BRANCH}" \
        --head "${BACKPORT_BRANCH}" \
        --title "Backport of ${PR_TITLE} (#${PR_NUMBER})" \
        --body "$BODY" \
        --label "${TARGET_BRANCH}"

    echo "✓ Backport PR created successfully"
    exit 0
else
    echo "✗ Cherry-pick failed"

    GIT_STATUS=$(git status)
    CONFLICTED_FILES=$(git diff --name-only --diff-filter=U || echo "Unable to determine conflicted files")
    git cherry-pick --abort || true

    BODY=$(cat <<EOF
Automatic backport to `${TARGET_BRANCH}` failed
### Conflict Details

<details>
<summary>Git Status</summary>

```
${GIT_STATUS}
```
</details>

### Conflicted Files
```
${CONFLICTED_FILES}
```

### Manual Backport Instructions

To manually backport this PR, run the following commands:

```bash
# Fetch the latest changes
git fetch origin

# Create a backport branch from ${TARGET_BRANCH}
git checkout -b backport-${PR_NUMBER} origin/${TARGET_BRANCH}

# Cherry-pick the commit
${CHERRY_PICK_CMD}

# Resolve conflicts in the listed files above
# Edit the conflicted files, then:
git add <resolved-files>
git cherry-pick --continue

# Push the backport branch
git push origin backport-${PR_NUMBER}

# Create a PR targeting ${TARGET_BRANCH}
gh pr create --base ${TARGET_BRANCH} --head backport-${PR_NUMBER} \
  --title "Backport of ${PR_TITLE} (#${PR_NUMBER})"

EOF

    gh pr comment "${PR_NUMBER}" --body "$BODY"
    echo "✗ Backport failed - comment posted to original PR"
    exit 1
fi
