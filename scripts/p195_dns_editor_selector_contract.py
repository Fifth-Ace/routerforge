"""P19.5: validate Svelte 5 editor selection and rejected-change rollback.

Static contract: a dropdown change must request the owner command and restore the
previous choice if the owner refuses to discard pending unsaved domain edits.
Importable from the source verifier and isolated negative unit tests.
"""
import re

CHOICE_HANDLER = re.compile(
    r"\bonchange\s*=\s*\{\s*event\s*=>\s*\{\s*"
    r"const\s+prior\s*=\s*chosen\s*;\s*"
    r"if\s*\(\s*send\s*\(\s*'editor-choice'\s*,\s*"
    r"event\.currentTarget\.value\s*\)\s*===\s*false\s*\)\s*"
    r"event\.currentTarget\.value\s*=\s*prior\s*;\s*\}\s*\}",
    flags=re.DOTALL,
)


def verify_editor_selector_contract(controls):
    """Raise specific failure for absent or unsafe selection behavior."""
    start_marker = '<select id="editorDNSChoice"'
    if controls.count(start_marker) != 1 or controls.count('id="editorDNSChoice"') != 1:
        raise AssertionError('P195_EDITOR_SELECTOR_NOT_UNIQUE')
    start = controls.index(start_marker)
    end = controls.find('</select>', start)
    if end < 0:
        raise AssertionError('P195_EDITOR_SELECTOR_NOT_CLOSED')
    selector = controls[start:end]
    if selector.count('value={chosen}') != 1:
        raise AssertionError('P195_EDITOR_CHOICE_VALUE_BINDING_MISSING')
    if not CHOICE_HANDLER.search(selector):
        raise AssertionError('P195_EDITOR_SELECTOR_REJECT_ROLLBACK_MISSING')
    return True
