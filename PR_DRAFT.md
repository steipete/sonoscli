# PR Draft: Play Mode Support

## Create PR Command
```bash
gh pr create --repo steipete/sonoscli --head STop211650:feature/play-modes-clean --base main --title "feat: add play mode support (shuffle/repeat)" --body "$(cat <<'EOF'
## Summary
- Add `GetTransportSettings()` and `SetPlayMode()` to the Sonos client for controlling playback modes
- New CLI command `sonos mode` with options: get, shuffle, shuffle-norepeat, repeat, repeat-one, normal
- Unit tests for new transport methods

## Usage
\`\`\`bash
sonos mode get --name "Kitchen"           # Show current mode
sonos mode shuffle --name "Kitchen"       # Shuffle with repeat
sonos mode shuffle-norepeat --name "Kitchen"  # Shuffle, no repeat
sonos mode repeat --name "Kitchen"        # Repeat all (no shuffle)
sonos mode repeat-one --name "Kitchen"    # Repeat single track
sonos mode normal --name "Kitchen"        # No shuffle, no repeat
\`\`\`

## Test plan
- [x] Unit tests pass (\`make test\`)
- [x] Tested against real Sonos speaker
- [x] Verified \`mode get\` returns correct state
- [x] Verified \`mode shuffle\` enables shuffle mode

🤖 Generated with [Claude Code](https://claude.com/claude-code)
EOF
)"
```

## Branch Info
- **Clean branch:** `feature/play-modes-clean` (based on upstream/main)
- **Target repo:** steipete/sonoscli
- **Base branch:** main

## Files Changed (4 files only)
- `internal/cli/mode.go` - New CLI command
- `internal/cli/root.go` - Register command (+1 line)
- `internal/sonos/avtransport.go` - New transport methods (+42 lines)
- `internal/sonos/avtransport_playmode_test.go` - Unit tests

## Checklist Before Submitting
- [ ] No beads files included
- [ ] No Apple Music code included
- [ ] No .claude/ files included
- [ ] Follows project conventions
- [ ] Tests pass
