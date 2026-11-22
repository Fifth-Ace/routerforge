import {highlightCustomStrategy} from './NfqwsCustomSyntax.js';
const esc=s=>String(s).replace(/[&<>"']/g,c=>({'&':'&amp;','<':'&lt;','>':'&gt;','"':'&quot;',"'":'&#39;'}[c]));
export function highlightLibraryConfig(source){
  return String(source??'').split(/\r?\n/).map(line=>{
    if(/^\s*#/.test(line))return `<span class="syn-comment">${esc(line)}</span>`;
    const m=/^(\s*)([a-z_][a-z0-9_]*)(\s*=)(.*)$/i.exec(line);
    return m?`${esc(m[1])}<span class="syn-key">${esc(m[2])}</span><span class="syn-operator">${esc(m[3])}</span>${highlightCustomStrategy(m[4])}`:
      highlightCustomStrategy(line);
  }).join('\n')||'&#8203;';
}
