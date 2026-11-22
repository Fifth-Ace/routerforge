#!/usr/bin/env python3
"""Negative-first tests for P19.5 Svelte 5 selector ownership contract."""
import unittest
from p195_dns_editor_selector_contract import verify_editor_selector_contract

GOOD = (
    '<select id="editorDNSChoice" aria-label="DNS-сервер для списков" '
    'value={chosen} disabled={!cards.some(c=>!c.native)} '
    'onchange={event => {const prior=chosen;if(send(\'editor-choice\',event.currentTarget.value)===false)'
    'event.currentTarget.value=prior;}}>'
    '<option value="">Выбери DNS</option></select>'
)

class SelectorContractTests(unittest.TestCase):
    def rejects(self, text, marker):
        with self.assertRaisesRegex(AssertionError, marker):
            verify_editor_selector_contract(text)

    def test_real_svelte5_handler_accepted(self):
        self.assertTrue(verify_editor_selector_contract(GOOD))

    def test_whitespace_only_changes_accepted(self):
        value = GOOD.replace('const prior=chosen;', 'const prior = chosen ;\n')
        value = value.replace(')===false)', ') === false )')
        self.assertTrue(verify_editor_selector_contract(value))

    def test_old_unconditional_handler_rejected(self):
        self.rejects(GOOD.replace(
            "{const prior=chosen;if(send('editor-choice',event.currentTarget.value)===false)event.currentTarget.value=prior;}",
            "send('editor-choice',event.currentTarget.value)"),
            'REJECT_ROLLBACK_MISSING')

    def test_cannot_drop_rollback(self):
        self.rejects(GOOD.replace('event.currentTarget.value=prior;', ''), 'REJECT_ROLLBACK_MISSING')

    def test_cannot_drop_false_comparison(self):
        self.rejects(GOOD.replace('===false', '!==false'), 'REJECT_ROLLBACK_MISSING')

    def test_cannot_revert_to_wrong_choice(self):
        self.rejects(GOOD.replace('value=prior;', 'value=chosen;'), 'REJECT_ROLLBACK_MISSING')

    def test_cannot_dispatch_wrong_command(self):
        self.rejects(GOOD.replace("'editor-choice'", "'default-change'"), 'REJECT_ROLLBACK_MISSING')

    def test_cannot_dispatch_wrong_value(self):
        self.rejects(GOOD.replace("send('editor-choice',event.currentTarget.value)",
            "send('editor-choice','old')"), 'REJECT_ROLLBACK_MISSING')

    def test_reject_duplicate_selector(self):
        self.rejects(GOOD+GOOD, 'SELECTOR_NOT_UNIQUE')

    def test_reject_selector_missing(self):
        self.rejects(GOOD.replace('id="editorDNSChoice"', 'id="other"'), 'SELECTOR_NOT_UNIQUE')

    def test_reject_missing_value_binding(self):
        self.rejects(GOOD.replace('value={chosen}', 'value={state.editing}'), 'VALUE_BINDING_MISSING')

    def test_wrong_selector_cannot_borrow_guard(self):
        marker=GOOD.index('onchange=')
        unsafe=GOOD[:marker]+"onchange={event => send('editor-choice',event.currentTarget.value)}>" + GOOD[GOOD.index('<option'):]
        self.rejects(unsafe+'<select id="another">'+GOOD[marker:GOOD.index('<option')]+"</select>", 'REJECT_ROLLBACK_MISSING')

    def test_unclosed_selector_rejected(self):
        self.rejects(GOOD.replace('</select>', ''), 'SELECTOR_NOT_CLOSED')

if __name__=='__main__':
    unittest.main()
