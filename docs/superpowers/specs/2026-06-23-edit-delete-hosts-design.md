# Edit & Delete Hosts — Design

Date: 2026-06-23

## Goal

Let users edit and delete hosts from the interactive picker, complementing the
existing "add host" flow.

## Decisions

- Delete shows a confirmation prompt before removing a host.
- Keybindings: `e` = edit, `d` = delete (both guarded against firing while filtering).
- Edit allows changing every field, including the name. The original entry is
  matched by its old name and replaced.

## Components

### 1. Config layer (`internal/hosts/config.go`)

Refactor the shared read/write logic into helpers, then add update/delete:

- `readConfig() (cfg config, path string, err error)` — resolve `~/.vibessh/hosts.yaml`,
  read + unmarshal. A missing file yields an empty config (no error) so callers can
  create it on write.
- `writeConfig(path string, cfg config) error` — marshal and write `0600`,
  creating the `~/.vibessh` dir as needed.
- `Append(node Node) error` — rewritten on top of the helpers.
- `Update(oldName string, node Node) error` — find the entry whose `Name == oldName`,
  replace it in place; return an error if no match.
- `Delete(name string) error` — remove the entry whose `Name == name`; return an
  error if no match.

Matching is by name (the app's de-facto identity, consistent with `matchNode` and
the sort order). First match wins.

### 2. Form add + edit (`internal/tui/form.go`)

- Add `mode` (add/edit) and `originalName string` to `formModel`.
- Keep `newFormModel()` for add; add `newEditFormModel(node Node)` that pre-fills all
  five inputs and records `originalName`.
- Title renders "Add Host" or "Edit Host" by mode.
- The form returns its result; persistence stays in the picker.

### 3. Picker wiring (`internal/tui/picker.go`)

- New key bindings `e` and `d`, guarded against filtering, shown in help next to `a`.
- `e`: open the form in edit mode pre-filled from the selected node.
- Form submit: add → `hosts.Append` + insert item; edit → `hosts.Update(originalName, result)`
  + replace the list item.
- `d`: enter `stateConfirmDelete`, showing `Delete host "<name>"? (y/n)`.
  `y` → `hosts.Delete(name)` + remove the list item; `n`/`esc` → back to list.
- Add `stateConfirmDelete` to the `pickerState` enum; route it in `Update`/`View`.

### 4. Error handling

Surface `Append`/`Update`/`Delete` write failures into the form/picker UI instead of
silently discarding them (the current add path ignores the error).

## Testing

- Unit tests for `hosts.Append`, `hosts.Update`, `hosts.Delete` against a temp `$HOME`:
  add → edit → delete round trip, plus not-found error paths.
- TUI remains manually verified (needs a TTY).

## Out of scope

- Reordering hosts, bulk operations, undo.
- Changing the YAML schema or identity model (name stays the identity).
