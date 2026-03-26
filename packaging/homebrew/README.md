# Homebrew Distribution

> by [neurosam.AI](https://neurosam.ai)

## User Installation

```bash
brew install neurosamAI/tap/tow
```

## Maintainer Setup

The actual formula lives in the **[neurosamAI/homebrew-tap](https://github.com/neurosamAI/homebrew-tap)** repo.

### Initial setup for homebrew-tap repo:

1. Create `Formula/tow.rb` (copy `tow.rb` from here as a starting template)
2. Add `.github/workflows/update-formula.yml` with this content:

```yaml
name: Update Formula
on:
  repository_dispatch:
    types: [update-formula]
permissions:
  contents: write
jobs:
  update:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - name: Update tow.rb formula
        run: |
          VERSION="${{ github.event.client_payload.version }}"
          # ... sed to replace version and SHA256 in Formula/tow.rb
      - name: Commit and push
        run: |
          git config user.name "github-actions[bot]"
          git config user.email "github-actions[bot]@users.noreply.github.com"
          git add Formula/tow.rb
          git commit -m "Update tow to v${{ github.event.client_payload.version }}"
          git push
```

3. Add `TAP_GITHUB_TOKEN` secret to the tow-cli repo (Settings → Secrets)

After this, every `git tag v*` push to tow-cli will automatically update the Homebrew formula.
