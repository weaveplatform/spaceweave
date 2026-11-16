// Plan catalogue for the picker. Modules load on demand so the page only
// fetches the plan it shows.
export const PLANS = [
  { id: 'office', name: 'Office', blurb: 'RoomSketcher sample · 19 × 11 m', load: () => import('./roomsketcher-office.js') },
  { id: 'clinic', name: 'Clinic', blurb: 'Physical therapy practice (SmartDraw sample)', load: () => import('./pt-clinic.js') },
  { id: 'suites', name: 'Suites', blurb: 'New Urban Suites, third floor', load: () => import('./urban-suites.js') },
  { id: 'corporate', name: 'Corporate', blurb: 'Open-plan floor with private offices', load: () => import('./corporate-floor.js') },
];

export const DEFAULT_PLAN = 'office';
