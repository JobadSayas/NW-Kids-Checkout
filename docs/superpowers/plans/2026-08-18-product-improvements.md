# Product Improvements Plan

Date: 2026-08-18

## Objective

Improve the NW Kids Checkout app from a product perspective across three user-selected focus areas:

1. **Core board: search + alerts** — search by name/code, new-child visual flash, board freshness indicator, show/hide confirmed toggle.
2. **Admin & ops** — location group CRUD, fetcher status view, delete events from the locations admin page.
3. **Metrics & history** — daily metrics endpoint (called/confirmed/avg time-to-confirm/manual count) plus an admin page.

Deliverable: enough detail in each task for a human or LLM to implement, test, and verify. Each phase ships independently.

## Product decisions (locked with user)

- Board search matches **first/last name OR security code**; child remains confirmed via the existing checkbox (no auto-confirm).
- New-child alert is a **visual flash only** (no audio).
- **No auto-clear of confirmed** children; keep all on the board. Add a **show/hide confirmed** toggle (client-side only, existing 12h window behavior unchanged; hidden children reappear when toggled back on or on reload).
- Metrics are **light**: per event + date — kids called, confirmed, avg minutes called→confirmed, and manual check-in count. One admin page, no charts.
- **Event deletion**: available on the locations admin page event rows; **cascade deletes** the event's locations, check-ins, and check windows in one transaction; requires a `confirm()` dialog first.

## Global constraints

- **No database migrations** are required for any phase (schema already has `location_groups`, `events.location_group_id`, `checkins.event_id`, `manual_checkins.created_at`, `events.last_checked_out_time`).
- Follow existing patterns in AGENTS.md: gofmt, testify (`require` for setup, `assert` for values), `db.PrepareTestDB()` for repo tests, `setupAuthedApp()` for controller tests, JSDOM + `window.__test` eval pattern for JS tests.
- All HTML/JS/CSS edits must keep the existing Tailwind look and mobile-first layout conventions.
- Run `npm run build:css` to rebuild the minified Tailwind CSS (`internal/web/static/css/tailwind.css`) whenever UI changes touch utility classes; the repo tracks the minified build, not the watch-mode output.
- Verification for every phase: `go fmt ./...`, `godotenv go test ./...`, `npm test`.
- Do not commit code; leave commits to the user.

---

## Task tracking

Every task below starts unchecked. **Mark a task done only when ALL of these are true:**

- The implementation described in the task is complete.
- The task's Tests section has been written (new test files created where specified) **and** all of them pass.
- The full test suite for that phase still passes (run the commands in the task's `### Tests` and the phase verification block).
- `go fmt ./...` has been run (Go changes only) and `gofmt` is clean.

To mark done, change the checkbox from `- [ ]` to `- [x]` next to the task name. Example: `- [x] Task 1.1 — Code + name search`.

Phase checkbox:
- [x] **Phase 1 — Core board** (complete when Tasks 1.1, 1.2, and 1.4 are checked)
- [x] **Phase 2 — Admin & ops** (complete when Tasks 2.2–2.4 are checked)
- [ ] **Phase 3 — Metrics & history** (complete when Tasks 3.1–3.3 are checked)

# Phase 1 — Core board: search + alerts

Files touched:
- `internal/web/static/pages/checkoutsv1/checkouts.html`
- `internal/web/static/pages/checkoutsv1/checkouts.js`
- `internal/web/static/pages/checkoutsv1/checkouts.test.js`

## Task 1.1 — Code + name search

- [x] **Task 1.1 — Code + name search** (mark done when implemented, tests written and passing, and Phase 1 suite green)

### HTML (`checkouts.html`)

Add a search input between the header and the main board area, so it is visible on the big screen and usable on mobile:

```html
<div class="relative z-10 flex justify-center px-4 pb-2">
  <input
    id="search-input"
    type="search"
    autocomplete="off"
    placeholder="Search by name or code"
    class="w-full max-w-md rounded-lg border border-slate-300 bg-white px-4 py-2 text-xl shadow-sm focus:border-slate-500 focus:outline-none"
  />
</div>
```

### JS (`checkouts.js`)

1. Add module state:
   ```js
   let searchQuery = '';
   ```
2. Add helper (place near other child helpers, e.g. after `getChildSignature`). This composes both the search filter (Task 1.1) and the hide-confirmed filter (Task 1.4); `hideConfirmed` is module state defined in Task 1.4 and defaults to `false`:
   ```js
   function getVisibleChildren() {
     let children = childrenData;
     if (hideConfirmed) {
       children = children.filter((child) => !child.checked_out_confirmed_at);
     }
     if (searchQuery) {
       const q = searchQuery.toLowerCase();
       children = children.filter((child) => {
         const name = `${child.first_name || ''} ${child.last_name || ''}`.toLowerCase();
         const code = (child.security_code || '').toLowerCase();
         return name.includes(q) || code.includes(q);
       });
     }
     return children;
   }
   ```
3. Add setter:
   ```js
   function setSearchQuery(query) {
     searchQuery = (query || '').trim();
     updateUI();
   }
   ```
4. In `updateUI()` (current code at `checkouts.js:372`) keep the existing `morphChildren` rendering path and only swap the data source + signature (include `hideConfirmed` so toggling re-renders):
   ```js
   const nowMs = Date.now();
   const visibleChildren = getVisibleChildren();
   const listSignature = hideConfirmed + '||' + searchQuery + '||' + visibleChildren.slice(0, 100).map(getChildSignature).join('||');
   if (dom.childrenList && listSignature !== lastListSignature) {
     const previousScrollTop = dom.childrenList.scrollTop;
     const markup = renderChildren(visibleChildren.slice(0, 100), nowMs, Boolean(searchQuery));
     morphChildren(dom.childrenList, markup);
     cacheChildTimeElements(dom.childrenList);
     requestAnimationFrame(() => { /* unchanged */ });
     lastListSignature = listSignature;
   }
   syncConfirmedStates();
   ```
   Do NOT switch to `innerHTML` — keep `morphChildren` (it preserves the DOM scroll behavior and perf).
5. Change `renderChildren` signature to `(children, nowMs, searchActive)` and update the empty-state (a third case for when hiding confirmed empties the board; `hideConfirmed` is module state):
   ```js
   if (children.length === 0) {
     if (searchActive) {
       return '<div class="text-center py-12 text-3xl text-slate-500">No matching children</div>';
     }
     if (hideConfirmed) {
       return '<div class="text-center py-12 text-3xl text-slate-500">No unconfirmed children</div>';
     }
     return '<div class="text-center py-12 text-3xl text-slate-500">No children called yet</div>';
   }
   ```
   Keep existing call sites working (they already pass `(children, nowMs)`).
6. Wire up input in the existing DOM init block (alongside the confirm checkbox delegation):
   ```js
   const searchInput = document.getElementById('search-input');
   if (searchInput) searchInput.addEventListener('input', () => setSearchQuery(searchInput.value));
   ```

### Tests (`checkouts.test.js`)

In `exposeInternals` add: `setSearchQuery`, `getVisibleChildren`. The harness loads the script with `window.eval` and exposes `window.__test`; tests use `loadWindow({ html })` where `html` may contain the DOM elements. Use the real element ids (`children-list`, `board-status`). Note `setDom()` currently only wires `dom.childrenList`; tests that exercise `updateUI` should pass an `html` containing `<ul id="children-list">` (see how `checkouts.test.js:78` passes html).

Add tests (mirroring existing `describe`/`it` style):
```js
it('filters visible children by name and code', () => {
  const window = loadWindow();
  window.__test.setChildrenData([
    { id: 'pc:1', first_name: 'Alice', last_name: 'Smith', security_code: '1234', source: 'planning_center' },
    { id: 'pc:2', first_name: 'Bob', last_name: 'Jones', security_code: '5678', source: 'planning_center' },
  ]);
  window.__test.setSearchQuery('ali');
  expect(window.__test.getVisibleChildren().map((c) => c.id)).toEqual(['pc:1']);
  window.__test.setSearchQuery('5678');
  expect(window.__test.getVisibleChildren().map((c) => c.id)).toEqual(['pc:2']);
  window.__test.setSearchQuery('');
  expect(window.__test.getVisibleChildren()).toHaveLength(2);
});

it('renders no-matching message for empty search results', () => {
  const window = loadWindow({ html: '<!doctype html><html><body><ul id="children-list"></ul></body></html>' });
  window.__test.setChildrenData([]);
  window.__test.setDom();
  window.__test.setSearchQuery('zzz');
  expect(window.document.getElementById('children-list').innerHTML).toContain('No matching children');
});
```

## Task 1.2 — New-child visual flash

- [x] **Task 1.2 — New-child visual flash** (mark done when implemented, tests written and passing, and Phase 1 suite green)

### HTML (`checkouts.html`)

Add a one-shot flash animation in the page `<style>` block:
```css
@keyframes new-child-flash {
  0% { background-color: #fef08a; }
  100% { background-color: #ffffff; }
}
.child-card-flash {
  animation: new-child-flash 3s ease-out;
}
```

### JS (`checkouts.js`)

1. Module state:
   ```js
   let knownChildIds = new Set();
   let flashChildIds = new Set();
   ```
2. Add helper:
   ```js
   function computeNewChildIds(children) {
     const currentIds = new Set(children.map(getChildId).filter(Boolean));
     const newlyAppeared = new Set();
     if (knownChildIds.size > 0) {
       currentIds.forEach((id) => {
         if (!knownChildIds.has(id)) newlyAppeared.add(id);
       });
     }
     knownChildIds = currentIds;
     return newlyAppeared;
   }
   ```
3. In `fetchChildrenData` after `childrenData = sortedData`:
   ```js
   const newIds = computeNewChildIds(childrenData);
   if (newIds.size > 0) {
     flashChildIds = newIds;
     setTimeout(() => {
       flashChildIds = new Set();
     }, 4000);
   }
   ```
4. In `renderChildren`, when building each child card add the class when flashing. The current card div is:
   ```js
   return `
       <div class="bg-white rounded-lg py-2.5 px-4 shadow-[0_0_10px_rgba(0,0,0,0.25)] flex flex-col justify-center">
   ```
   Change to:
   ```js
   const flashClass = flashChildIds.has(childId) ? ' child-card-flash' : '';
   return `
       <div class="bg-white rounded-lg py-2.5 px-4 shadow-[0_0_10px_rgba(0,0,0,0.25)] flex flex-col justify-center${flashClass}">
   ```
   (`childId` already exists in scope and is escaped.)

### Tests (`checkouts.test.js`)

In `exposeInternals` add: `computeNewChildIds`, `getFlashChildIds`, `setFlashChildIds`.

```js
it('computeNewChildIds seeds on first call and detects later additions', () => {
  const window = loadWindow();
  const first = window.__test.computeNewChildIds([{ id: 'pc:1' }]);
  expect(first.size).toBe(0);
  const second = window.__test.computeNewChildIds([{ id: 'pc:1' }, { id: 'pc:2' }]);
  expect(Array.from(second)).toEqual(['pc:2']);
});

it('renders child-card-flash class for flashing ids', () => {
  const window = loadWindow();
  const html = window.renderChildren(
    [{ id: 'pc:1', first_name: 'A', last_name: 'B', security_code: '1', source: 'planning_center', checked_out_at: new Date().toISOString() }],
    Date.now(),
    false,
  );
  expect(html).not.toContain('child-card-flash');
  window.__test.setFlashChildIds(new Set(['pc:1']));
  const flashing = window.renderChildren(
    [{ id: 'pc:1', first_name: 'A', last_name: 'B', security_code: '1', source: 'planning_center', checked_out_at: new Date().toISOString() }],
    Date.now(),
    false,
  );
  expect(flashing).toContain('child-card-flash');
});
```

Note: `flashChildIds` is only assigned inside `fetchChildrenData`; `setFlashChildIds` is exposed purely for the render assertion above. When wiring `fetchChildrenData`, add the `computeNewChildIds` + `setTimeout` logic immediately after `childrenData = sortedData` (before `updateUI()`).

## Task 1.4 — Show/hide confirmed toggle

- [x] **Task 1.4 — Show/hide confirmed toggle** (mark done when implemented, tests written and passing, and Phase 1 suite green)

### HTML (`checkouts.html`)

Add a toggle button in the header (keep it compact; visible on the big screen). Default state is **show confirmed** (`aria-pressed="false"`); pressing it hides confirmed children, pressing again shows them:
```html
<button
  id="toggle-confirmed-button"
  type="button"
  aria-pressed="false"
  class="rounded-lg border border-slate-300 bg-white px-3 py-1.5 text-sm font-medium text-slate-700 shadow-sm hover:bg-slate-50"
>
  Hide confirmed
</button>
```

### JS (`checkouts.js`)

1. Add module state next to `searchQuery`:
   ```js
   let hideConfirmed = false;
   ```
2. Add helper that sets the toggle state, updates the button label, and re-renders (signature includes `hideConfirmed`, so `updateUI` re-renders):
   ```js
   function setHideConfirmed(hidden) {
     hideConfirmed = Boolean(hidden);
     const button = document.getElementById('toggle-confirmed-button');
     if (button) {
       button.textContent = hideConfirmed ? 'Show confirmed' : 'Hide confirmed';
       button.setAttribute('aria-pressed', String(hideConfirmed));
     }
     updateUI();
   }
   ```
3. Wire up in the DOM init block:
   ```js
   const toggleButton = document.getElementById('toggle-confirmed-button');
   if (toggleButton) toggleButton.addEventListener('click', () => setHideConfirmed(!hideConfirmed));
   ```
4. `getVisibleChildren()` (from Task 1.1) already filters confirmed when `hideConfirmed` is true, and `renderChildren` shows the "No unconfirmed children" empty state.

### Tests (`checkouts.test.js`)

In `exposeInternals` add: `setHideConfirmed`, `getVisibleChildren` (already added in Task 1.1), `getChildrenData`.

```js
it('setHideConfirmed filters confirmed children from view without deleting them', () => {
  const window = loadWindow();
  window.__test.setChildrenData([
    { id: 'pc:1', first_name: 'A', last_name: 'B', security_code: '1', source: 'planning_center', checked_out_confirmed_at: '2026-08-18T10:00:00Z' },
    { id: 'pc:2', first_name: 'C', last_name: 'D', security_code: '2', source: 'planning_center', checked_out_confirmed_at: null },
  ]);
  window.__test.setHideConfirmed(true);
  const visible = window.__test.getVisibleChildren();
  expect(visible.map((c) => c.id)).toEqual(['pc:2']);
  expect(window.__test.getChildrenData()).toHaveLength(2);
});

it('setHideConfirmed(false) shows confirmed children again', () => {
  const window = loadWindow();
  window.__test.setChildrenData([
    { id: 'pc:1', first_name: 'A', last_name: 'B', security_code: '1', source: 'planning_center', checked_out_confirmed_at: '2026-08-18T10:00:00Z' },
    { id: 'pc:2', first_name: 'C', last_name: 'D', security_code: '2', source: 'planning_center', checked_out_confirmed_at: null },
  ]);
  window.__test.setHideConfirmed(true);
  window.__test.setHideConfirmed(false);
  expect(window.__test.getVisibleChildren()).toHaveLength(2);
});

it('toggle button label reflects state', () => {
  const window = loadWindow({ html: '<!doctype html><html><body><button id="toggle-confirmed-button" aria-pressed="false">Hide confirmed</button></body></html>' });
  window.__test.setHideConfirmed(true);
  const button = window.document.getElementById('toggle-confirmed-button');
  expect(button.textContent).toBe('Show confirmed');
  expect(button.getAttribute('aria-pressed')).toBe('true');
  window.__test.setHideConfirmed(false);
  expect(button.textContent).toBe('Hide confirmed');
  expect(button.getAttribute('aria-pressed')).toBe('false');
});

it('renders no-unconfirmed message when hiding confirmed empties the board', () => {
  const window = loadWindow();
  window.__test.setChildrenData([
    { id: 'pc:1', first_name: 'A', last_name: 'B', security_code: '1', source: 'planning_center', checked_out_confirmed_at: '2026-08-18T10:00:00Z' },
  ]);
  window.__test.setHideConfirmed(true);
  const html = window.renderChildren(window.__test.getVisibleChildren(), Date.now(), false);
  expect(html).toContain('No unconfirmed children');
});
```

## Phase 1 verification

- `go fmt ./...` (no Go changes expected but run for safety).
- `npm test` (or `npx vitest run internal/web/static/pages/checkoutsv1/checkouts.test.js`).
- Manual: `make db-reset db-seed`, `make checkout-fetcher`, `make web` in dev; type in search box; confirm a child flashes; stop the fetcher and watch the status line turn red; toggle "Hide confirmed"/"Show confirmed".

---

# Phase 2 — Admin & ops

Files touched:
- `internal/repo/location/location.go`
- `internal/repo/location/mock_repo.go`
- `internal/repo/location/location_test.go`
- `internal/controllers/locationgroupv1/location_group.go`
- `internal/controllers/locationgroupv1/location_group_test.go` (new)
- `internal/web/static/pages/admin/locations.html`
- `internal/web/static/pages/admin/locations.js`
- `internal/web/static/pages/admin/locations.test.js`
- `internal/web/static/pages/admin/fetcher-status.html` (new)
- `internal/web/static/pages/admin/fetcher-status.js` (new)
- `internal/web/static/pages/admin/fetcher-status.test.js` (new)
- `internal/controllers/admin/locations.go`
- `internal/web/static/pages/admin/index.html`
- `internal/repo/event/event.go`
- `internal/repo/event/mock_repo.go`
- `internal/repo/event/event_test.go`
- `internal/actions/eventlocation/delete.go` (new)
- `internal/actions/eventlocation/delete_test.go` (new)
- `internal/controllers/eventv1/event.go`
- `internal/controllers/eventv1/event_test.go`

## Task 2.2 — Location group CRUD (create, rename, delete-when-unused)

- [x] **Task 2.2 — Location group CRUD** (mark done when implemented, tests written and passing, and Phase 2 suite green)

The sqlite repo already has `CreateLocationGroup` but it is **not** on the `Repo` interface or mock. Add `UpdateLocationGroup` and `DeleteLocationGroup`.

### Repo (`internal/repo/location/location.go`)

1. Add sentinel:
   ```go
   var ErrLocationGroupInUse = errors.New("location group is in use")
   ```
2. Add to `Repo` interface:
   ```go
   UpdateLocationGroup(ctx context.Context, lg LocationGroup) error
   DeleteLocationGroup(ctx context.Context, id int64) error
   ```
3. Implement on `sqliteRepo`:
   ```go
   func (r *sqliteRepo) UpdateLocationGroup(ctx context.Context, lg LocationGroup) error {
       result, err := squirrel.Update("location_groups").
           Set("name", lg.Name).
           Where(squirrel.Eq{"id": lg.ID}).
           RunWith(r.db).
           ExecContext(ctx)
       if err != nil {
           return fmt.Errorf("updating location group: %w", err)
       }
       rows, err := result.RowsAffected()
       if err != nil {
           return fmt.Errorf("reading updated rows: %w", err)
       }
       if rows == 0 {
           return repo.ErrNotFound
       }
       return nil
   }

   func (r *sqliteRepo) DeleteLocationGroup(ctx context.Context, id int64) error {
       for _, table := range []string{"locations", "events"} {
           count, err := r.countWhere(ctx, table, squirrel.Eq{"location_group_id": id})
           if err != nil {
               return fmt.Errorf("checking %s references: %w", table, err)
           }
           if count > 0 {
               return ErrLocationGroupInUse
           }
       }
       result, err := squirrel.Delete("location_groups").
           Where(squirrel.Eq{"id": id}).
           RunWith(r.db).
           ExecContext(ctx)
       if err != nil {
           return fmt.Errorf("deleting location group: %w", err)
       }
       rows, err := result.RowsAffected()
       if err != nil {
           return fmt.Errorf("reading deleted rows: %w", err)
       }
       if rows == 0 {
           return repo.ErrNotFound
       }
       return nil
   }
   ```
   Add a small private `countWhere` helper (or inline the two COUNT queries; prefer a helper consistent with the file's style).

### Mock (`internal/repo/location/mock_repo.go`)

Add fields `UpdateLocationGroupFunc`, `DeleteLocationGroupFunc` and wire both methods to call them, returning `repo.ErrNotFound` / `ErrLocationGroupInUse` defaults if the funcs are nil, consistent with the existing mock style.

### Repo tests (`location_test.go`)

```go
func Test_sqliteRepo_UpdateLocationGroup(t *testing.T) {
    // create via CreateLocationGroup, rename, assert new name returned by ListLocationGroups
}

func Test_sqliteRepo_DeleteLocationGroup_in_use(t *testing.T) {
    // create a location group, create a location referencing it (see existing test fixtures)
    // assert errors.Is(err, ErrLocationGroupInUse)
}

func Test_sqliteRepo_DeleteLocationGroup_unused(t *testing.T) {
    // create group, delete, assert repo.ErrNotFound on second delete
}

func Test_sqliteRepo_UpdateLocationGroup_not_found(t *testing.T) {
    // update id 999999 -> errors.Is(err, repo.ErrNotFound)
}
```
Use `db.PrepareTestDB()` + `t.Cleanup` and `t.Context()` per repo conventions.

### Controller (`internal/controllers/locationgroupv1/location_group.go`)

In `RegisterRoutes`, add an admin group:
```go
adminGroup := app.Group("/v1/admin/location_groups")
adminGroup.Use(middleware.AuthRequired(controller.sessionStore, "admin"))
adminGroup.Post("", controller.PostCreateLocationGroup)
adminGroup.Patch("/:id", controller.PatchUpdateLocationGroup)
adminGroup.Delete("/:id", controller.DeleteLocationGroup)
```
Verify the package already imports `internal/middleware`; add if missing.

Handlers (reuse existing input/output conversion helpers if present, else add `LocationGroupInput{Name}`):
```go
func (controller *Controller) PostCreateLocationGroup(c *fiber.Ctx) error {
    var input LocationGroupInput
    if err := json.Unmarshal(c.Body(), &input); err != nil {
        return fiber.NewError(fiber.StatusBadRequest, "invalid JSON")
    }
    input.Name = strings.TrimSpace(input.Name)
    if input.Name == "" {
        return fiber.NewError(fiber.StatusBadRequest, "name is required")
    }
    created, err := controller.repo.CreateLocationGroup(c.Context(), location.LocationGroup{Name: input.Name})
    if err != nil {
        return fmt.Errorf("creating location group: %w", err)
    }
    return c.Status(fiber.StatusCreated).JSON(converter(created))
}

func (controller *Controller) PatchUpdateLocationGroup(c *fiber.Ctx) error {
    id, err := c.ParamsInt("id")
    if err != nil {
        return fiber.NewError(fiber.StatusBadRequest, "invalid location group id")
    }
    var input LocationGroupInput
    if err := json.Unmarshal(c.Body(), &input); err != nil {
        return fiber.NewError(fiber.StatusBadRequest, "invalid JSON")
    }
    input.Name = strings.TrimSpace(input.Name)
    if input.Name == "" {
        return fiber.NewError(fiber.StatusBadRequest, "name is required")
    }
    if err := controller.repo.UpdateLocationGroup(c.Context(), location.LocationGroup{ID: int64(id), Name: input.Name}); err != nil {
        if errors.Is(err, repo.ErrNotFound) {
            return fiber.NewError(fiber.StatusNotFound, "location group not found")
        }
        return fmt.Errorf("updating location group: %w", err)
    }
    return c.SendStatus(fiber.StatusNoContent)
}

func (controller *Controller) DeleteLocationGroup(c *fiber.Ctx) error {
    id, err := c.ParamsInt("id")
    if err != nil {
        return fiber.NewError(fiber.StatusBadRequest, "invalid location group id")
    }
    if err := controller.repo.DeleteLocationGroup(c.Context(), int64(id)); err != nil {
        if errors.Is(err, location.ErrLocationGroupInUse) {
            return fiber.NewError(fiber.StatusBadRequest, "location group is in use")
        }
        if errors.Is(err, repo.ErrNotFound) {
            return fiber.NewError(fiber.StatusNotFound, "location group not found")
        }
        return fmt.Errorf("deleting location group: %w", err)
    }
    return c.SendStatus(fiber.StatusNoContent)
}
```

### Controller tests

Create a new `internal/controllers/locationgroupv1/location_group_test.go`. There is no existing test file in this package. The controller takes `*sql.DB` (see `NewController(db *sql.DB, ...)`), so mirror the pattern from `internal/controllers/locationv1/location_api_test.go`: use `db.PrepareTestDB()` + `t.Cleanup`, `setupAuthedApp()` with the admin role, and a real repo. Use `require` for setup and `assert` for value checks.

Note: the current `locationgroupv1.NewController` signature is `NewController(db *sql.DB, sessionStore session.Storer)` (from `location_group.go:20`), so keep that signature when adding admin routes.
- POST creates a group (201, body has id+name).
- POST empty name → 400.
- PATCH renames (204).
- PATCH unknown id → 404.
- DELETE in-use → 400.
- DELETE unused → 204.
- DELETE unknown → 404.

### UI (`admin/locations.html`, `admin/locations.js`)

1. In `locations.html`, add a "Location Groups" card between the header and the locations list:
   ```html
   <section class="rounded-xl border border-slate-200 bg-white p-4 shadow-sm">
     <h2 class="mb-3 text-lg font-semibold text-slate-800">Location Groups</h2>
     <div id="location-groups-list" class="mb-3 space-y-2"></div>
     <div class="flex items-center gap-2">
       <input
         id="new-group-name"
         type="text"
         placeholder="New group name"
         class="flex-1 rounded-lg border border-slate-300 px-3 py-2 text-sm focus:border-slate-500 focus:outline-none"
       />
       <button
         id="add-group-button"
         class="rounded-lg bg-slate-800 px-3 py-2 text-sm font-medium text-white hover:bg-slate-700"
       >
         Add Group
       </button>
     </div>
   </section>
   ```
2. In `locations.js`:
   - Add `renderLocationGroups()` that renders each group with an inline rename input + Save and Delete buttons, called from `loadData` after groups load and after any mutation:
     ```js
     function renderLocationGroups() {
       const list = document.getElementById('location-groups-list');
       if (!list) return;
       list.innerHTML = locationGroups
         .map(
           (group) => `
             <div class="flex items-center gap-2" data-group-id="${group.id}">
               <input type="text" value="${escapeHtml(group.name)}"
                      class="group-name-input flex-1 rounded-lg border border-slate-300 px-3 py-1.5 text-sm focus:border-slate-500 focus:outline-none"
                      data-group-id="${group.id}" />
               <button class="save-group-button rounded border border-slate-300 px-2 py-1 text-xs font-medium hover:bg-slate-50"
                       data-group-id="${group.id}">Save</button>
               <button class="delete-group-button rounded border border-slate-300 px-2 py-1 text-xs font-medium text-red-600 hover:bg-red-50"
                       data-group-id="${group.id}">Delete</button>
             </div>`,
         )
         .join('');
     }
     ```
   - Add event delegation on the section (or per-button listeners) for Save/Delete; add listener on `#add-group-button` → `createLocationGroup(name)`.
   - Add async helpers:
     ```js
     async function createLocationGroup(name) {
       const response = await fetch(`${API_URL}/v1/admin/location_groups`, {
         method: 'POST',
         headers: { 'Content-Type': 'application/json' },
         body: JSON.stringify({ name }),
       });
       if (!response.ok) throw new Error(`failed to create group (${response.status})`);
     }
     async function updateLocationGroup(id, name) {
       const response = await fetch(`${API_URL}/v1/admin/location_groups/${id}`, {
         method: 'PATCH',
         headers: { 'Content-Type': 'application/json' },
         body: JSON.stringify({ name }),
       });
       if (!response.ok) throw new Error(`failed to rename group (${response.status})`);
     }
     async function deleteLocationGroup(id) {
       const response = await fetch(`${API_URL}/v1/admin/location_groups/${id}`, { method: 'DELETE' });
       if (!response.ok) {
         const data = await response.json().catch(() => ({}));
         throw new Error(data.message || `failed to delete group (${response.status})`);
       }
     }
     ```
   - After each mutation succeeds: re-run `loadData()` to refresh group dropdowns, then `renderLocationGroups()`.
   - Use existing `escapeHtml` helper if present; add one otherwise.
   - Wire Save to read the sibling input value; Delete calls `deleteLocationGroup` and surfaces in-use errors in a status element (mirror the page's existing error display).
   - Expose new helpers via the file's `window.__test` block: `renderLocationGroups`, `createLocationGroup`, `updateLocationGroup`, `deleteLocationGroup`, `setLocationGroups`.

### UI tests (`locations.test.js`)

```js
test('createLocationGroup posts to /v1/admin/location_groups', async () => {
  // fetch mock records url/method/body; assert 201 handled and loadData re-run (assert fetch to groups endpoint happened again)
});

test('deleteLocationGroup surfaces in-use message', async () => {
  // fetch mock returns { ok: false, status: 400, json: async () => ({ message: 'location group is in use' }) }
  // assert thrown error message contains 'location group is in use'
});

test('renderLocationGroups renders rows with inputs', () => {
  window.__test.setLocationGroups([{ id: 1, name: 'K-3' }]);
  // assert list contains 'K-3' and Save/Delete buttons
});
```

## Task 2.3 — Fetcher status view

- [x] **Task 2.3 — Fetcher status view** (mark done when implemented, tests written and passing, and Phase 2 suite green)

### Controller (`internal/controllers/admin/locations.go`)

Add route + handler in `RegisterRoutes`:
```go
adminGroup.Get("/fetcher-status", controller.GetFetcherStatus)
```
Handler serves `internal/web/static/pages/admin/fetcher-status.html` using the same `c.SendStream(...)`/embedded FS helper the other admin pages use.

### HTML (`fetcher-status.html`) — new

Mirror `locations.html` structure (topbar + card). Include:
- Intro text explaining "how long since each event's checkouts were last fetched".
- `<table>` with columns: Event, Auto-fetch, Last fetched, Age.
- `tbody` id `fetcher-status-body`.
- `<script src="fetcher-status.js?v=dev"></script>` (dev) — match how other pages reference assets (see `locations.html`).

### JS (`fetcher-status.js`) — new

```js
const API_URL = '';
const STALE_THRESHOLD_MS = 15 * 60 * 1000;

async function loadEvents() {
  const response = await fetch(`${API_URL}/v1/events`);
  if (!response.ok) throw new Error(`failed to load events (${response.status})`);
  return response.json();
}

function formatAge(ms) {
  if (ms < 60 * 1000) return `${Math.max(0, Math.floor(ms / 1000))}s`;
  if (ms < 60 * 60 * 1000) return `${Math.floor(ms / 60000)}m`;
  return `${Math.floor(ms / 3600000)}h`;
}

function renderEvents(events) {
  const body = document.getElementById('fetcher-status-body');
  const now = Date.now();
  const rows = events
    .map((event) => {
      const last = event.last_checked_out_time ? new Date(event.last_checked_out_time).getTime() : null;
      const stale = last !== null && now - last > STALE_THRESHOLD_MS;
      const autoFetch = event.auto_fetch;
      let status;
      if (last === null) {
        status = '<span class="rounded bg-slate-100 px-2 py-0.5 text-xs text-slate-600">never</span>';
      } else if (stale) {
        status = '<span class="rounded bg-red-100 px-2 py-0.5 text-xs font-medium text-red-700">stale</span>';
      } else {
        status = '<span class="rounded bg-green-100 px-2 py-0.5 text-xs font-medium text-green-700">ok</span>';
      }
      return `
        <tr class="border-b border-slate-100">
          <td class="px-4 py-3 text-slate-800">${escapeHtml(event.name)}</td>
          <td class="px-4 py-3">
            ${autoFetch ? '<span class="text-xs text-slate-600">yes</span>' : '<span class="text-xs text-slate-400">no</span>'}
          </td>
          <td class="px-4 py-3 text-slate-600">${last !== null ? new Date(last).toLocaleString() : '—'}</td>
          <td class="px-4 py-3">${last !== null ? formatAge(now - last) : '—'} ${status}</td>
        </tr>`;
    })
    .join('');
  body.innerHTML = rows;
}

function escapeHtml(value) {
  return String(value)
    .replace(/&/g, '&amp;')
    .replace(/</g, '&lt;')
    .replace(/>/g, '&gt;')
    .replace(/"/g, '&quot;');
}

async function main() {
  const statusEl = document.getElementById('fetcher-status-error');
  try {
    const events = await loadEvents();
    renderEvents(events);
    if (statusEl) statusEl.textContent = '';
  } catch (error) {
    if (statusEl) statusEl.textContent = error.message;
  }
}

document.addEventListener('DOMContentLoaded', main);

// test hooks (mirror existing pattern)
window.__test = { renderEvents, loadEvents, formatAge };
```

Add `<div id="fetcher-status-error" class="hidden ..."></div>` (or reuse an error element pattern from `locations.html`).

### Admin landing (`index.html`)

Add a "Fetcher status" link/card alongside the other admin links (replace/extend the placeholder block), pointing to `/admin/fetcher-status`.

### Tests (`fetcher-status.test.js`) — new

Mirror the JSDOM harness:
```js
test('renderEvents marks stale events', () => {
  const events = [
    { name: 'Kids', auto_fetch: true, last_checked_out_time: new Date(Date.now() - 20 * 60 * 1000).toISOString() },
    { name: 'Tweens', auto_fetch: false, last_checked_out_time: new Date().toISOString() },
  ];
  const body = document.createElement('tbody');
  body.id = 'fetcher-status-body';
  document.body.appendChild(body);
  window.__test.renderEvents(events);
  const html = body.innerHTML;
  expect(html).toContain('stale');
  expect(html).toContain('ok');
});
```

## Task 2.4 — Delete events from admin (locations page event rows)

- [x] **Task 2.4 — Delete events from admin** (mark done when implemented, tests written and passing, and Phase 2 suite green)

### Semantics (locked with user)

- Delete button lives on the **locations admin page** (`/admin/locations`) on each event header row, next to "Check Windows".
- **Cascade delete all** dependent data in **one transaction**: the event's locations, checkins, and check windows.
- **Confirm first** with a `confirm()` dialog naming the event and stating that its locations/check-ins will be removed.

### Repo (`internal/repo/event/event.go`)

Add to `Repo` interface and `sqliteRepo` (single-row delete; `repo.ErrNotFound` when the row is absent):
```go
DeleteEvent(ctx context.Context, id int64) error
```

Implementation:
```go
func (r *sqliteRepo) DeleteEvent(ctx context.Context, id int64) error {
    res, err := squirrel.Delete("events").
        Where(squirrel.Eq{"id": id}).
        RunWith(r.db).
        ExecContext(ctx)
    if err != nil {
        return fmt.Errorf("deleting event: %w", err)
    }
    rowsAffected, _ := res.RowsAffected()
    if rowsAffected == 0 {
        return repo.ErrNotFound
    }
    return nil
}
```

### Mock (`internal/repo/event/mock_repo.go`)

Add `DeleteEventFunc func(ctx context.Context, id int64) error` and `DeleteEventFuncCallCount atomic.Int64`; implement `DeleteEvent` to increment the counter, call the func if set, otherwise `panic("MockRepo.DeleteEvent not implemented")`.

### Action (`internal/actions/eventlocation/delete.go`) — new file

Mirror the `CreateEventWithLocations` pattern (transaction via `repo.WithTx`). Because `locations.event_id` is `NOT NULL`, `checkins` link via both `event_id` and `location_id`, and `PRAGMA foreign_keys` is not enabled, the action deletes dependents explicitly and atomically:

```go
package eventlocation

import (
    "context"
    "database/sql"

    "kids-checkin/internal/repo"
    "kids-checkin/internal/repo/event"

    "github.com/Masterminds/squirrel"
)

func DeleteEventWithDependents(ctx context.Context, db *sql.DB, eventID int64) error {
    return repo.WithTx(ctx, db, func(tx *sql.Tx) error {
        if _, err := squirrel.Delete("event_check_windows").
            Where(squirrel.Eq{"event_id": eventID}).
            RunWith(tx).
            ExecContext(ctx); err != nil {
            return err
        }

        if _, err := squirrel.Delete("checkins").
            Where(squirrel.Or{
                squirrel.Eq{"event_id": eventID},
                squirrel.Expr("location_id IN (SELECT id FROM locations WHERE event_id = ?)", eventID),
            }).
            RunWith(tx).
            ExecContext(ctx); err != nil {
            return err
        }

        if _, err := squirrel.Delete("locations").
            Where(squirrel.Eq{"event_id": eventID}).
            RunWith(tx).
            ExecContext(ctx); err != nil {
            return err
        }

        eventRepo := event.NewRepo(tx)
        if err := eventRepo.DeleteEvent(ctx, eventID); err != nil {
            return err
        }
        return nil
    })
}
```

Ordering note: check windows and checkins are removed before locations/events so no orphaned references remain at any intermediate point (not FK-enforced, but keeps the DB consistent if the tx were inspected mid-flight).

### Controller (`internal/controllers/eventv1/event.go`)

Add the route in `RegisterRoutes` on the admin group:
```go
adminGroup.Delete("/:id", controller.DeleteEvent)
```

Handler (the controller already holds `db *sql.DB`; import `kids-checkin/internal/actions/eventlocation` if not present):
```go
func (controller *Controller) DeleteEvent(c *fiber.Ctx) error {
    log := middleware.GetLogger(c)

    eventID, err := strconv.ParseInt(c.Params("id"), 10, 64)
    if err != nil {
        return fiber.NewError(fiber.StatusBadRequest, "invalid event id")
    }

    if err := eventlocation.DeleteEventWithDependents(c.Context(), controller.db, eventID); err != nil {
        if errors.Is(err, repo.ErrNotFound) {
            return fiber.NewError(fiber.StatusNotFound, "event not found")
        }
        log.ErrorContext(c.Context(), "failed to delete event", slog.Int64("event_id", eventID), slog.String("error", err.Error()), slog.Any("err", err))
        return err
    }

    log.InfoContext(c.Context(), "deleted event", slog.Int64("event_id", eventID))
    return c.SendStatus(fiber.StatusNoContent)
}
```

### UI (`admin/locations.html`, `admin/locations.js`)

1. In `locations.js` `renderLocations`, inside the event header row's last cell (next to the "Check Windows" button), add a Delete button:
   ```js
   const deleteButton = document.createElement('button');
   deleteButton.type = 'button';
   deleteButton.className = 'inline-flex cursor-pointer items-center gap-2 rounded-md border border-red-300 px-3 py-1.5 text-sm font-semibold text-red-700 shadow-sm transition hover:bg-red-50';
   deleteButton.textContent = 'Delete';
   deleteButton.addEventListener('click', () => handleDeleteEvent(event));
   ```
   Append it to the same `<td>` as the Check Windows button (wrap both in a flex container if needed for spacing).
2. Add the handler (reuse existing `setPageStatus`, `clearPageStatus`):
   ```js
   function handleDeleteEvent(event) {
     const confirmed = window.confirm(
       `Delete "${event.name}"?\n\nThis will permanently remove the event and its locations, check-ins, and check windows.`,
     );
     if (!confirmed) return;
     clearPageStatus();
     fetch(`${API_URL}/v1/admin/events/${event.id}`, { method: 'DELETE' })
       .then((response) => {
         if (!response.ok) {
           return response.text().then((message) => {
             throw new Error(message || `Failed to delete event (${response.status})`);
           });
         }
         setPageStatus(`Deleted "${event.name}".`, 'success');
         loadData();
       })
       .catch((error) => {
         setPageStatus(`Failed to delete "${event.name}": ${error.message}`, 'error');
       });
   }
   ```
   Note: `loadData()` re-renders so the deleted event disappears.
3. Expose for tests via `window.__test`: `renderLocations` (already exposed), `handleDeleteEvent`, `setEvents`, `setLocations`.

### Tests

#### Repo tests (`internal/repo/event/event_test.go`)

```go
func Test_sqliteRepo_DeleteEvent(t *testing.T) {
    testDB, cleanup, err := db.PrepareTestDB()
    require.NoError(t, err, "Failed to prepare test DB")
    t.Cleanup(cleanup)

    eventRepo := NewRepo(testDB)

    created, err := eventRepo.CreateEvent(t.Context(), Event{Name: "Kids Service", PlanningCenterID: "pc_evt_del"})
    require.NoError(t, err)
    require.NotZero(t, created.ID)

    err = eventRepo.DeleteEvent(t.Context(), created.ID)
    require.NoError(t, err)

    _, err = eventRepo.GetEventByID(t.Context(), created.ID)
    require.ErrorIs(t, err, repo.ErrNotFound)

    err = eventRepo.DeleteEvent(t.Context(), created.ID)
    require.ErrorIs(t, err, repo.ErrNotFound)
}
```

#### Action tests (`internal/actions/eventlocation/delete_test.go`) — new file

Seed an event, two locations, one checkin (via both `event_id` and a `location_id`), and one check window; call `DeleteEventWithDependents`; assert the event, locations, checkins, and check windows are gone, and a second call returns `repo.ErrNotFound`. Mirror `create_test.go` fixture style (`squirrel.Insert` with `RunWith(testDB)`).

#### Controller tests (`internal/controllers/eventv1/event_test.go`)

- `DELETE /v1/admin/events/:id` → 204 when the event exists.
- `DELETE /v1/admin/events/:id` for unknown id → 404.
- Request without admin role → 401/403 (auth middleware behavior).

#### UI tests (`internal/web/static/pages/admin/locations.test.js`)

```js
it('renders a delete button per event row', () => {
    const window = loadWindow();
    window.__test.setEvents([{ id: 1, name: 'Kids Check-in', auto_fetch: false, location_group_id: null }]);
    window.__test.setLocations([]);
    window.__test.renderLocations();
    const button = window.document.querySelector('.event-row [data-delete-event-id="1"]');
    expect(button).not.toBeNull();
});

it('delete event sends DELETE and reloads on confirm', async () => {
    const calls = [];
    const fetchImpl = async (url, opts) => {
        calls.push({ url, opts });
        if (url.includes('/v1/locations') || url.includes('/v1/location_groups') || url.includes('/v1/events')) {
            return { ok: true, status: 200, json: async () => [], text: async () => '' };
        }
        return { ok: true, status: 204, text: async () => '' };
    };
    const window = loadWindow(fetchImpl);
    const confirmSpy = vitest.spyOn(window, 'confirm').mockReturnValue(true);
    await window.__test.handleDeleteEvent({ id: 1, name: 'Kids Check-in' });
    expect(confirmSpy).toHaveBeenCalled();
    expect(calls.some(c => c.url === '/v1/admin/events/1' && c.opts.method === 'DELETE')).toBe(true);
});

it('delete event skips request when confirm is declined', async () => {
    const calls = [];
    const fetchImpl = async (url) => {
        calls.push(url);
        return { ok: true, status: 200, json: async () => [], text: async () => '' };
    };
    const window = loadWindow(fetchImpl);
    vitest.spyOn(window, 'confirm').mockReturnValue(false);
    await window.__test.handleDeleteEvent({ id: 1, name: 'Kids Check-in' });
    expect(calls.some(url => url === '/v1/admin/events/1')).toBe(false);
});
```

For the first two tests, add `data-delete-event-id="${event.id}"` to the delete button in `renderLocations` so tests can select it. Note the `loadData()` reload in the success path fires additional fetches for `/v1/locations`, `/v1/location_groups`, and `/v1/events` — the fetch stub must handle those (as above). Also `handleDeleteEvent` uses `window.confirm`; in JSDOM, set it up in `loadWindow` (or stub per test with `vitest.spyOn`). Existing `loadWindow` in `locations.test.js` may need a default `window.confirm = () => true`.

## Phase 2 verification

- `go fmt ./...`
- `godotenv go test ./internal/repo/location ./internal/repo/event ./internal/actions/eventlocation`
- `godotenv go test ./internal/controllers/locationgroupv1 ./internal/controllers/eventv1`
- `npx vitest run internal/web/static/pages/admin/locations.test.js internal/web/static/pages/admin/fetcher-status.test.js`
- Manual: admin → locations; add/rename/delete a group (delete blocked while used); delete an event and confirm its locations/checkins/check windows disappear; admin → fetcher status shows per-event staleness.

---

# Phase 3 — Metrics & history

Files touched:
- `internal/repo/metrics/metrics.go` (new)
- `internal/repo/metrics/mock_repo.go` (new)
- `internal/repo/metrics/metrics_test.go` (new)
- `internal/controllers/metricsv1/metrics.go` (new)
- `internal/controllers/metricsv1/metrics_test.go` (new)
- `internal/controllers/server.go`
- `internal/web/static/pages/admin/metrics.html` (new)
- `internal/web/static/pages/admin/metrics.js` (new)
- `internal/web/static/pages/admin/metrics.test.js` (new)
- `internal/controllers/admin/locations.go`
- `internal/web/static/pages/admin/index.html`

## Task 3.1 — Metrics repo

- [x] **Task 3.1 — Metrics repo** (mark done when implemented, tests written and passing, and Phase 3 suite green)

### Repo (`internal/repo/metrics/metrics.go`) — new package

```go
package metrics

import (
    "context"
    "fmt"
    "sort"
    "time"

    "kids-checkin/internal/repo"

    "github.com/Masterminds/squirrel"
)

type DailyMetric struct {
    Date              string
    EventName         string
    Called            int
    Confirmed         int
    Unconfirmed       int
    AvgConfirmMinutes float64
    ManualCount       int
}

type Filter struct {
    Days int
}

type Repo interface {
    ListDailyMetrics(ctx context.Context, filter Filter) ([]DailyMetric, error)
}

type sqliteRepo struct {
    db repo.DBTX
}

func NewRepo(database repo.DBTX) Repo {
    return &sqliteRepo{db: database}
}
```

Implementation:

```go
func (r *sqliteRepo) ListDailyMetrics(ctx context.Context, filter Filter) ([]DailyMetric, error) {
    days := filter.Days
    if days <= 0 {
        days = 14
    }
    since := time.Now().UTC().AddDate(0, 0, -days)

    pcRows, err := squirrel.Select(
        "date(checkins.checked_out_at) AS day",
        "COALESCE(events.name, 'Unknown') AS event_name",
        "COUNT(*) AS called",
        "COALESCE(SUM(CASE WHEN checkins.checked_out_confirmed_at IS NOT NULL THEN 1 ELSE 0 END), 0) AS confirmed",
        "COALESCE(AVG(CASE WHEN checkins.checked_out_confirmed_at IS NOT NULL THEN (julianday(checkins.checked_out_confirmed_at) - julianday(checkins.checked_out_at)) * 24 * 60 END), 0) AS avg_minutes",
    ).
        From("checkins").
        LeftJoin("locations ON locations.id = checkins.location_id").
        LeftJoin("events ON events.id = COALESCE(checkins.event_id, locations.event_id)").
        Where(squirrel.GtOrEq{"checkins.checked_out_at": since}).
        GroupBy("day", "event_name").
        OrderBy("day DESC, event_name ASC").
        RunWith(r.db).
        QueryContext(ctx)
    if err != nil {
        return nil, fmt.Errorf("querying checkin metrics: %w", err)
    }
    defer pcRows.Close()

    daily := []DailyMetric{}
    for pcRows.Next() {
        var dm DailyMetric
        var day string
        if err := pcRows.Scan(&day, &dm.EventName, &dm.Called, &dm.Confirmed, &dm.AvgConfirmMinutes); err != nil {
            return nil, fmt.Errorf("scanning checkin metrics: %w", err)
        }
        dm.Date = day
        dm.Unconfirmed = dm.Called - dm.Confirmed
        daily = append(daily, dm)
    }
    if err := pcRows.Err(); err != nil {
        return nil, fmt.Errorf("iterating checkin metrics: %w", err)
    }

    manualRows, err := squirrel.Select(
        "date(created_at) AS day",
        "COUNT(*) AS count",
    ).
        From("manual_checkins").
        Where(squirrel.GtOrEq{"created_at": since}).
        GroupBy("day").
        OrderBy("day DESC").
        RunWith(r.db).
        QueryContext(ctx)
    if err != nil {
        return nil, fmt.Errorf("querying manual metrics: %w", err)
    }
    defer manualRows.Close()

    manualByDay := map[string]int{}
    for manualRows.Next() {
        var day string
        var count int
        if err := manualRows.Scan(&day, &count); err != nil {
            return nil, fmt.Errorf("scanning manual metrics: %w", err)
        }
        manualByDay[day] = count
    }
    if err := manualRows.Err(); err != nil {
        return nil, fmt.Errorf("iterating manual metrics: %w", err)
    }

    // Merge manual counts into the matching day rows; add rows for manual-only days.
    for i := range daily {
        if count, ok := manualByDay[daily[i].Date]; ok {
            daily[i].ManualCount = count
            delete(manualByDay, daily[i].Date)
        }
    }
    for day, count := range manualByDay {
        daily = append(daily, DailyMetric{Date: day, EventName: "Manual Check-Ins", ManualCount: count})
    }

    // Deterministic sort: day DESC, event_name ASC.
    sort.SliceStable(daily, func(i, j int) bool {
        if daily[i].Date != daily[j].Date {
            return daily[i].Date > daily[j].Date
        }
        return daily[i].EventName < daily[j].EventName
    })

    return daily, nil
}
```

Add the `sort` import.

### Mock (`mock_repo.go`) — new

```go
type MockRepo struct {
    ListDailyMetricsFunc func(ctx context.Context, filter Filter) ([]DailyMetric, error)
}

func (m *MockRepo) ListDailyMetrics(ctx context.Context, filter Filter) ([]DailyMetric, error) {
    if m.ListDailyMetricsFunc == nil {
        return nil, fmt.Errorf("ListDailyMetrics not implemented")
    }
    return m.ListDailyMetricsFunc(ctx, filter)
}
```

### Repo tests (`metrics_test.go`) — new

Use `db.PrepareTestDB()` and insert via the raw DB (or existing repos):
- Insert a checkin with `checked_out_at` today and `checked_out_confirmed_at` 5 minutes later; assert `Called=1`, `Confirmed=1`, `AvgConfirmMinutes≈5`, `Unconfirmed=0`.
- Insert a checkin today unconfirmed; assert `Called=1`, `Confirmed=0`, `Unconfirmed=1`, `AvgConfirmMinutes=0`.
- Insert a manual checkin today; assert `ManualCount=1` merged into the PC row for the same day (or a `Manual Check-Ins` row if no PC checkins that day).
- Filter with `Days: 0` returns at least the today rows; a `Days: 1` filter excludes older data (insert one checkin with `checked_out_at` 3 days ago and assert it is absent).
- Use `t.Context()`, `require` for setup, `assert` for values.

## Task 3.2 — Metrics controller

- [ ] **Task 3.2 — Metrics controller** (mark done when implemented, tests written and passing, and Phase 3 suite green)

### Controller (`internal/controllers/metricsv1/metrics.go`) — new package

```go
package metricsv1

import (
    "fmt"
    "strconv"

    "github.com/gofiber/fiber/v2"

    "kids-checkin/internal/middleware"
    "kids-checkin/internal/repo/metrics"
    "kids-checkin/internal/session"
)

type Controller struct {
    repo         metrics.Repo
    sessionStore session.Storer
}

func NewController(repo metrics.Repo, sessionStore session.Storer) *Controller {
    return &Controller{repo: repo, sessionStore: sessionStore}
}

func (controller *Controller) RegisterRoutes(app fiber.Router) {
    group := app.Group("/v1/admin/metrics")
    group.Use(middleware.AuthRequired(controller.sessionStore, "admin"))
    group.Get("", controller.GetMetrics)
}

type DailyMetricResponse struct {
    Date              string  `json:"date"`
    EventName         string  `json:"event_name"`
    Called            int     `json:"called"`
    Confirmed         int     `json:"confirmed"`
    Unconfirmed       int     `json:"unconfirmed"`
    AvgConfirmMinutes float64 `json:"avg_confirm_minutes"`
    ManualCount       int     `json:"manual_count"`
}

type MetricsResponse struct {
    Days  int                   `json:"days"`
    Daily []DailyMetricResponse `json:"daily"`
}

func (controller *Controller) GetMetrics(c *fiber.Ctx) error {
    days := 14
    if raw := c.Query("days"); raw != "" {
        parsed, err := strconv.Atoi(raw)
        if err != nil || parsed < 1 || parsed > 90 {
            return fiber.NewError(fiber.StatusBadRequest, "days must be an integer between 1 and 90")
        }
        days = parsed
    }

    daily, err := controller.repo.ListDailyMetrics(c.Context(), metrics.Filter{Days: days})
    if err != nil {
        return fmt.Errorf("listing daily metrics: %w", err)
    }

    response := MetricsResponse{Days: days, Daily: make([]DailyMetricResponse, 0, len(daily))}
    for _, dm := range daily {
        response.Daily = append(response.Daily, DailyMetricResponse{
            Date:              dm.Date,
            EventName:         dm.EventName,
            Called:            dm.Called,
            Confirmed:         dm.Confirmed,
            Unconfirmed:       dm.Unconfirmed,
            AvgConfirmMinutes: math.Round(dm.AvgConfirmMinutes*100) / 100,
            ManualCount:       dm.ManualCount,
        })
    }
    return c.JSON(response)
}
```

Add `math` import.

### Register (`internal/controllers/server.go`)

Wire the controller with the metrics repo and register routes (mirror existing controller wiring, e.g. `checkinv1`):
```go
metricsRepo := metrics.NewRepo(db)
metricsV1 := metricsv1.NewController(metricsRepo, sessionStore)
metricsV1.RegisterRoutes(app)
```

### Controller tests (`metrics_test.go`) — new

Mirror `setupAuthedApp()` used elsewhere:
- `GET /v1/admin/metrics` returns 200 with `daily` array.
- `GET /v1/admin/metrics?days=abc` returns 400.
- `GET /v1/admin/metrics?days=0` returns 400.
- Without admin role, returns 401/403 (auth middleware behavior).

## Task 3.3 — Metrics admin page

- [ ] **Task 3.3 — Metrics admin page** (mark done when implemented, tests written and passing, and Phase 3 suite green)

### Admin controller (`internal/controllers/admin/locations.go`)

Add route + handler:
```go
adminGroup.Get("/metrics", controller.GetMetrics)
```
Handler serves `internal/web/static/pages/admin/metrics.html` via the same embedded-FS helper.

### HTML (`metrics.html`) — new

Mirror `locations.html` structure. Include:
- A days selector:
  ```html
  <select id="metrics-days" class="rounded-lg border border-slate-300 px-3 py-2 text-sm">
    <option value="7">Last 7 days</option>
    <option value="14" selected>Last 14 days</option>
    <option value="30">Last 30 days</option>
  </select>
  ```
- `<table>` with columns: Date, Event, Called, Confirmed, Unconfirmed, Avg confirm (min), Manual.
- `tbody` id `metrics-body`, error element, and `<script src="metrics.js?v=dev"></script>`.

### JS (`metrics.js`) — new

```js
const API_URL = '';

async function loadMetrics(days) {
  const response = await fetch(`${API_URL}/v1/admin/metrics?days=${days}`);
  if (!response.ok) {
    const data = await response.json().catch(() => ({}));
    throw new Error(data.message || `failed to load metrics (${response.status})`);
  }
  return response.json();
}

function renderMetrics(data) {
  const body = document.getElementById('metrics-body');
  if (!body) return;
  body.innerHTML = data.daily
    .map(
      (m) => `
        <tr class="border-b border-slate-100">
          <td class="px-4 py-3 text-slate-600">${escapeHtml(m.date)}</td>
          <td class="px-4 py-3 text-slate-800">${escapeHtml(m.event_name)}</td>
          <td class="px-4 py-3 text-slate-800">${m.called}</td>
          <td class="px-4 py-3 text-slate-800">${m.confirmed}</td>
          <td class="px-4 py-3 text-slate-600">${m.unconfirmed}</td>
          <td class="px-4 py-3 text-slate-600">${m.avg_confirm_minutes}</td>
          <td class="px-4 py-3 text-slate-600">${m.manual_count}</td>
        </tr>`,
    )
    .join('');
  if (data.daily.length === 0) {
    body.innerHTML = '<tr><td colspan="7" class="px-4 py-8 text-center text-slate-500">No data yet.</td></tr>';
  }
}

async function main() {
  const statusEl = document.getElementById('metrics-error');
  const daysEl = document.getElementById('metrics-days');
  const load = async () => {
    try {
      const data = await loadMetrics(daysEl ? daysEl.value : 14);
      renderMetrics(data);
      if (statusEl) statusEl.textContent = '';
    } catch (error) {
      if (statusEl) statusEl.textContent = error.message;
    }
  };
  if (daysEl) daysEl.addEventListener('change', load);
  await load();
}

document.addEventListener('DOMContentLoaded', main);

window.__test = { renderMetrics, loadMetrics };
```

Include `escapeHtml` (same helper as fetcher-status; could be shared, but duplicating in each page JS matches current repo convention).

### Admin landing (`index.html`)

Add a "Metrics" link/card pointing to `/admin/metrics`.

### UI tests (`metrics.test.js`) — new

```js
test('renderMetrics renders rows and empty state', () => {
  const body = document.createElement('tbody');
  body.id = 'metrics-body';
  document.body.appendChild(body);
  window.__test.renderMetrics({
    days: 14,
    daily: [
      { date: '2026-08-18', event_name: 'Kids', called: 5, confirmed: 4, unconfirmed: 1, avg_confirm_minutes: 3.5, manual_count: 2 },
    ],
  });
  expect(body.innerHTML).toContain('Kids');
  expect(body.innerHTML).toContain('3.5');

  window.__test.renderMetrics({ days: 14, daily: [] });
  expect(body.innerHTML).toContain('No data yet.');
});

test('loadMetrics builds URL with days param', async () => {
  const calls = [];
  const fetchImpl = async (url) => {
    calls.push(url);
    return { ok: true, json: async () => ({ days: 7, daily: [] }) };
  };
  // harness injects fetchImpl; assert calls[0] contains 'days=7'
});
```

## Phase 3 verification

- `go fmt ./...`
- `godotenv go test ./internal/repo/metrics ./internal/controllers/metricsv1`
- `npx vitest run internal/web/static/pages/admin/metrics.test.js`
- Manual: run with some seed data, admin → metrics; verify numbers match `SELECT` counts in `db/structure.sql` tables; toggle the days selector.

---

# Final integration & acceptance

1. Confirm every task checkbox above is `[x]`: `- [x] Task 1.1 …` through `- [x] Task 3.3 …`, and the three phase checkboxes are checked.
2. Run full suite: `make test` (runs `godotenv go test ./...` and `npm test`).
3. Run `go fmt ./...`.
4. Manual smoke: `make db-reset db-seed` → `make checkout-fetcher` → `make web`; verify board search/flash/freshness/show-hide-confirmed toggle; admin sync button; location group CRUD; delete an event and confirm its locations/checkins/check windows disappear; fetcher status; metrics page.
5. If any feature is reverted, uncheck its task box, document why, and add a follow-up.
6. Leave the repo uncommitted; present a summary to the user.

## Risks / notes

- The board search input is a new top-of-page element; on the large-screen board it sits between header and list — confirm it doesn't overlap the fixed header on `lg:` breakpoints (use `relative z-10` and test at 1080p).
- `computeNewChildIds` intentionally seeds on first load to avoid flashing the entire initial list.
- The show/hide confirmed toggle filters the client-side view only; `childrenData` is untouched, and hidden children reappear when toggled back on or on the next reload. The existing 12h window behavior is unchanged.
- The `events` join in metrics uses `COALESCE(checkins.event_id, locations.event_id)`; if a location has no event and the checkin has no event, `event_name` falls back to `Unknown`.
- All new admin pages must be reachable from `admin/index.html`.