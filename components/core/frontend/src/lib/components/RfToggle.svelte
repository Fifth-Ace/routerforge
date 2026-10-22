<script>
  // Visual-only control: persistence, confirmation and failures stay with the caller.
  export let checked = false;
  export let disabled = false;
  export let busy = false;
  export let ariaLabel = 'Toggle setting';
  export let onToggle = () => {};
</script>

<button
  type="button"
  class="rf-toggle"
  class:checked
  role="switch"
  aria-label={ariaLabel}
  aria-checked={checked}
  aria-busy={busy}
  disabled={disabled || busy}
  onclick={() => onToggle(!checked)}
>
  <span class="rf-toggle-status" aria-hidden="true">{checked ? 'ON' : 'OFF'}</span>
  <span class="rf-toggle-track" aria-hidden="true"><span class="rf-toggle-thumb"></span></span>
</button>

<style>
  button.rf-toggle {
    appearance: none;
    box-sizing: border-box;
    display: inline-flex;
    align-items: center;
    justify-content: flex-end;
    gap: 9px;
    width: auto;
    min-width: 0;
    min-height: 28px;
    padding: 1px;
    border: 0;
    border-radius: 8px;
    background: transparent;
    color: var(--rf-text, #f5f7fa);
    cursor: pointer;
    font: inherit;
    white-space: nowrap;
  }
  .rf-toggle .rf-toggle-status {
    display: inline-flex;
    align-items: center;
    justify-content: center;
    min-width: 47px;
    height: 23px;
    padding: 0 9px;
    border: 1px solid var(--rf-border-strong, #36414d);
    border-radius: 999px;
    background: var(--rf-surface-2, #171b21);
    color: var(--rf-muted, #8d98a4);
    font: 600 11px/1 var(--font-mono, 'Roboto Mono', Consolas, monospace);
    letter-spacing: .02em;
  }
  .rf-toggle .rf-toggle-track {
    box-sizing: border-box;
    display: block;
    position: relative;
    width: 46px;
    height: 24px;
    flex: 0 0 46px;
    border: 1px solid var(--rf-border-strong, #36414d);
    border-radius: 999px;
    background: var(--rf-surface-2, #171b21);
    transition: background .16s ease, border-color .16s ease;
  }
  .rf-toggle .rf-toggle-thumb {
    display: block;
    position: absolute;
    left: 3px;
    top: 3px;
    width: 16px;
    height: 16px;
    border-radius: 50%;
    background: #8d98a4;
    box-shadow: 0 1px 2px rgba(0, 0, 0, .35);
    transition: transform .16s ease, background .16s ease;
  }
  button.rf-toggle.checked .rf-toggle-status {
    border-color: color-mix(in srgb, var(--rf-good, #2ea043) 45%, var(--rf-border-strong, #36414d));
    background: color-mix(in srgb, var(--rf-good, #2ea043) 9%, var(--rf-surface-2, #171b21));
    color: var(--rf-good, #2ea043);
  }
  button.rf-toggle.checked .rf-toggle-track {
    border-color: var(--rf-good, #2ea043);
    background: color-mix(in srgb, var(--rf-good, #2ea043) 26%, var(--rf-surface-2, #171b21));
  }
  button.rf-toggle.checked .rf-toggle-thumb {
    transform: translateX(22px);
    background: var(--rf-good, #2ea043);
  }
  button.rf-toggle:hover:not(:disabled) .rf-toggle-track {
    border-color: var(--rf-accent, #38bdf8);
  }
  button.rf-toggle:focus-visible {
    outline: 2px solid var(--rf-accent, #38bdf8);
    outline-offset: 3px;
  }
  button.rf-toggle:disabled {
    opacity: .5;
    cursor: not-allowed;
  }
  button.rf-toggle[aria-busy="true"] .rf-toggle-track { opacity: .6; }
  @media (prefers-reduced-motion: reduce) {
    .rf-toggle .rf-toggle-track,
    .rf-toggle .rf-toggle-thumb { transition: none; }
  }
</style>
