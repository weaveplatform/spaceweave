import { polygonArea, pointInPolygon } from './geometry.js';
import { deskTargets } from './desks.js';

export function describeSelection(plan, selection) {
  if (!plan || !selection) return null;
  if (selection.type === 'room') {
    const room = plan.rooms.find(r => r.id === selection.id);
    return room ? { ...selection, label: room.name, roomId: room.id,
      detail: `${room.kind === 'zone' ? 'Open-plan zone' : 'Room'} · ${polygonArea(room.polygon).toFixed(1)} m²` } : null;
  }
  if (selection.type !== 'desk') return null;
  const desk = deskTargets(plan.furniture).find(d => d.id === selection.id);
  if (!desk) return null;
  const item = plan.furniture[desk.index];
  const room = plan.rooms.filter(r => pointInPolygon([item.x, item.y], r.polygon))
    .sort((a, b) => polygonArea(a.polygon) - polygonArea(b.polygon))[0];
  const size = item.w > 0 && item.d > 0 ? `${item.w.toFixed(2)} × ${item.d.toFixed(2)} m` : 'Desk';
  return { ...selection, label: desk.label, roomId: room?.id,
    detail: [size, room?.name].filter(Boolean).join(' · ') };
}
