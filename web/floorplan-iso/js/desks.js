export function deskTargets(furniture) {
  let number = 0;
  return furniture.flatMap((item, index) => {
    if (item.type !== 'desk') return [];
    const label = typeof item.label === 'string' ? item.label.trim() : '';
    return [{ id: `desk-${index}`, index, label: `Desk ${++number}${label ? ` · ${label}` : ''}` }];
  });
}

// One delegated interaction per scene; hovering never rebuilds the geometry.
// Source identity survives replacement prisms when the skin or build changes.
export function createDeskHighlight({ container, svg, targets, show, hide, onSelect }) {
  const desks = new Map(targets.map(desk => [desk.id, desk]));
  const parts = new Map(), listeners = [];
  let hovered = null, focused = null, selected = null;
  let pointerEvent = null, focusEvent = null;

  function idAt(node) {
    const face = node?.closest?.('[data-desk-id]');
    return face && container.contains(face) && desks.has(face.dataset.deskId) ? face.dataset.deskId : null;
  }
  function paint(node, id) {
    const over = hovered === id || focused === id, chosen = selected === id;
    node.classList.toggle('desk-hovered', over);
    node.classList.toggle('desk-selected', chosen);
    if (over || chosen) {
      node.style.setProperty('fill', `color-mix(in srgb, var(--accent) ${chosen ? 55 : 35}%, ${node.__data__.fill})`);
    } else if (node.style.getPropertyValue('fill')) {
      node.style.removeProperty('fill');
      if (!node.getAttribute('style')) node.removeAttribute('style');
    }
    if (node.hasAttribute('data-desk-target')) node.setAttribute('aria-pressed', String(chosen));
  }
  function repaint(...ids) {
    for (const id of new Set(ids)) for (const node of parts.get(id) ?? []) paint(node, id);
  }
  function present() {
    const id = hovered ?? focused, event = hovered ? pointerEvent : focusEvent;
    if (id && event) show(event, desks.get(id), selected === id);
    else hide();
  }
  function hover(id, event) {
    const before = hovered;
    hovered = id; pointerEvent = event;
    if (before !== id) repaint(before, id);
    present();
  }
  function focus(id, event) {
    const before = focused;
    focused = id; focusEvent = event;
    if (before !== id) repaint(before, id);
    present();
  }
  function select(id, notify = true) {
    const before = selected;
    selected = id;
    if (before !== id) repaint(before, id);
    present();
    if (before !== id && notify) onSelect?.(desks.get(id) ?? null);
  }
  function listen(target, type, handler) {
    target.addEventListener(type, handler);
    listeners.push(() => target.removeEventListener(type, handler));
  }
  listen(container, 'pointerover', event => hover(idAt(event.target), event));
  listen(container, 'pointermove', event => {
    const id = idAt(event.target);
    if (id) hover(id, event);
  });
  listen(container, 'pointerout', event => {
    const id = idAt(event.relatedTarget);
    if (id !== hovered) hover(id, id ? event : null);
  });
  listen(container, 'focusin', event => focus(idAt(event.target), event));
  listen(container, 'focusout', event => {
    const id = idAt(event.relatedTarget);
    if (id !== focused) focus(id, null);
  });
  listen(svg, 'click', event => {
    if (event.defaultPrevented) return;
    const id = idAt(event.target);
    select(id === selected ? null : id);
    if (id) {
      const target = [...(parts.get(id) ?? [])].find(node => node.hasAttribute('data-desk-target'));
      target?.focus({ preventScroll: true });
      present();
    } else if (idAt(svg.ownerDocument.activeElement)) svg.ownerDocument.activeElement.blur();
  });
  listen(container, 'keydown', event => {
    const id = idAt(event.target);
    if (id && (event.key === 'Enter' || event.key === ' ')) {
      event.preventDefault();
      if (!event.repeat) select(id === selected ? null : id);
    }
  });
  listen(svg.ownerDocument, 'keydown', event => {
    if (event.key !== 'Escape' || event.defaultPrevented || svg.ownerDocument.querySelector('dialog[open]')) return;
    select(null);
    hover(null, null);
    if (idAt(svg.ownerDocument.activeElement)) svg.ownerDocument.activeElement.blur();
  });

  function unbind(node) {
    const id = node.getAttribute('data-desk-id');
    if (!id) return;
    const nodes = parts.get(id);
    nodes?.delete(node);
    if (nodes?.size === 0) {
      parts.delete(id);
      if (hovered === id) hover(null, null);
      if (focused === id) focus(null, null);
    }
    for (const name of ['data-desk-id', 'data-desk-target', 'role', 'tabindex', 'aria-label', 'aria-pressed']) node.removeAttribute(name);
    node.classList.remove('desk-hovered', 'desk-selected');
    if (node.style.getPropertyValue('fill')) {
      node.style.removeProperty('fill');
      if (!node.getAttribute('style')) node.removeAttribute('style');
    }
  }
  return {
    setSelected(id) { select(id, false); },
    bind(node, face) {
      const old = node.getAttribute('data-desk-id');
      if (old && old !== face.deskId) unbind(node);
      if (!desks.has(face.deskId)) return;
      node.setAttribute('data-desk-id', face.deskId);
      if (!parts.has(face.deskId)) parts.set(face.deskId, new Set());
      parts.get(face.deskId).add(node);
      if (face.deskTarget) {
        node.setAttribute('data-desk-target', '');
        node.setAttribute('role', 'button');
        node.setAttribute('tabindex', '0');
        node.setAttribute('aria-label', desks.get(face.deskId).label);
      } else {
        for (const name of ['data-desk-target', 'role', 'tabindex', 'aria-label', 'aria-pressed']) node.removeAttribute(name);
      }
      paint(node, face.deskId);
    },
    refresh(node, face) {
      if (face.deskId && (hovered === face.deskId || focused === face.deskId || selected === face.deskId)) paint(node, face.deskId);
    },
    unbind,
    destroy() {
      for (const dispose of listeners) dispose();
      parts.clear();
      hovered = focused = selected = pointerEvent = focusEvent = null;
      hide();
    },
  };
}
