# Frontend architecture

## Decision

DNS Monitor no longer uses client-side SPA routing as its primary navigation model.

Each top-level area is a real document served by the router:

- `/` — Overview
- `/servers` — DNS servers
- `/routing` — routing diagnostics
- `/monitoring` — live monitoring
- `/tools` — tools
- `/catalog` — Marketplace
- `/settings` — settings

Navigation uses normal `<a href>` document navigation. There is no `pushState`
router and unknown routes are no longer silently rewritten to the root index.

## Live data

A document may still poll JSON APIs, but polling must not destroy and recreate the
page root. `web/js/morph.js` incrementally synchronizes the existing DOM:

- focused inputs remain the same DOM nodes;
- scrollable live areas remain the same DOM nodes;
- open `<details>` / planner dialogs stay open;
- only changed text, attributes and child structures are updated.

This is the migration boundary for all future pages and modules:

> APIs may update widgets; they must not replace the document.

## Future split

The shared controller remains temporarily so all existing functions keep working
during migration. New functionality should use page-scoped entry modules. Once all
top-level pages have page-specific controllers, legacy global routing/controller
code can be removed completely.
