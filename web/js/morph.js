// Incremental DOM updater for DNS Monitor.
//
// The old frontend replaced #pageWorkspace with innerHTML on every refresh.
// That destroyed focus, scroll positions, <details> state and any future
// component-local state. This synchronizer keeps existing nodes whenever the
// structure is compatible and only updates changed attributes/text/children.

function nodeKey(node) {
  if (!(node instanceof Element)) return '';
  if (node.id) return `id:${node.id}`;
  for (const attr of ['data-key','data-port','data-client-ip','data-live-flow','data-preserve-scroll']) {
    const value = node.getAttribute(attr);
    if (value) return `${attr}:${value}`;
  }
  return '';
}

function syncAttributes(current, next) {
  const active = document.activeElement === current;
  const preserveOpen = current instanceof HTMLDetailsElement && current.open;

  for (const attr of [...current.attributes]) {
    if (!next.hasAttribute(attr.name)) {
      if (preserveOpen && attr.name === 'open') continue;
      if (active && ['value','checked','selected'].includes(attr.name)) continue;
      current.removeAttribute(attr.name);
    }
  }

  for (const attr of [...next.attributes]) {
    if (preserveOpen && attr.name === 'open') continue;
    if (active && ['value','checked','selected'].includes(attr.name)) continue;
    if (current.getAttribute(attr.name) !== attr.value) {
      current.setAttribute(attr.name, attr.value);
    }
  }

  if (preserveOpen) current.open = true;

  if (!active) {
    if (current instanceof HTMLInputElement && next instanceof HTMLInputElement) {
      if (current.type !== 'file' && current.value !== next.value) current.value = next.value;
      if (current.checked !== next.checked) current.checked = next.checked;
    } else if (current instanceof HTMLTextAreaElement && next instanceof HTMLTextAreaElement) {
      if (current.value !== next.value) current.value = next.value;
    } else if (current instanceof HTMLSelectElement && next instanceof HTMLSelectElement) {
      if (current.value !== next.value) current.value = next.value;
    }
  }
}

function syncNode(current, next) {
  if (!current || !next) return;

  if (current.nodeType !== next.nodeType) {
    current.replaceWith(next.cloneNode(true));
    return;
  }

  if (current.nodeType === Node.TEXT_NODE || current.nodeType === Node.COMMENT_NODE) {
    if (current.nodeValue !== next.nodeValue) current.nodeValue = next.nodeValue;
    return;
  }

  if (!(current instanceof Element) || !(next instanceof Element)) return;

  if (current.tagName !== next.tagName) {
    current.replaceWith(next.cloneNode(true));
    return;
  }

  syncAttributes(current, next);
  syncChildren(current, next);
}

function syncChildren(currentParent, nextParent) {
  const currentChildren = [...currentParent.childNodes];
  const nextChildren = [...nextParent.childNodes];

  const keyed = new Map();
  for (const child of currentChildren) {
    const key = nodeKey(child);
    if (key) keyed.set(key, child);
  }

  let cursor = currentParent.firstChild;

  for (const nextChild of nextChildren) {
    const key = nodeKey(nextChild);
    let currentChild = key ? keyed.get(key) : cursor;

    if (key && currentChild && currentChild !== cursor) {
      currentParent.insertBefore(currentChild, cursor);
    }

    if (!currentChild) {
      currentParent.appendChild(nextChild.cloneNode(true));
      cursor = null;
      continue;
    }

    const compatible =
      currentChild.nodeType === nextChild.nodeType &&
      (!(currentChild instanceof Element) ||
        !(nextChild instanceof Element) ||
        currentChild.tagName === nextChild.tagName);

    if (!compatible) {
      const replacement = nextChild.cloneNode(true);
      currentParent.insertBefore(replacement, currentChild);
      const old = currentChild;
      cursor = old.nextSibling;
      old.remove();
      continue;
    }

    syncNode(currentChild, nextChild);
    cursor = currentChild.nextSibling;
  }

  while (cursor) {
    const next = cursor.nextSibling;
    cursor.remove();
    cursor = next;
  }
}

export function morphHTML(root, html) {
  const template = document.createElement('template');
  template.innerHTML = html;
  syncChildren(root, template.content);
}
