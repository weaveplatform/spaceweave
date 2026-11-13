// Physical therapy clinic traced by hand from SmartDraw's "Physical Therapy in a
// Physicians Office" sample (2108×1300 px PNG). All coordinates are metres on
// wall centrelines: x runs left→right, y runs top→bottom, matching the source
// image, so the top-down view lines up with the original without any flip.
//
// Calibration: the sheet has no dimensions, so the scale comes from standard
// objects. The gym's double door and the waiting-room entrance double door are
// both 81–83 px wide (2 × 0.91 m leaves → 44.8–45.3 px/m); the treadmills are
// 95 px long (≈2.1 m at 45 px/m, a typical deck plus motor hood); the exam
// tables are 99 × 35 px (2.2 × 0.78 m). The single interior doors are drawn at
// 38–40 px, which reads as 0.85 m leaves at this scale. 45 px/m fits all of these
// within a few percent. The sanitary-ware symbols are drawn oversize (a toilet is
// ~54 px, which would make it 1.2 m) so they were not used.
//
// Wall centrelines were found by scanning the image for dark runs 4–12 px thick;
// walls and openings are those pixel positions divided by 45. Furniture is given
// in source pixels and converted with fromPx so it can be checked against the
// symbols directly.

import {
  THICKNESS, fromPx, door, doubleDoor, win, opening, rect, E, S, W, N, toward,
  desk, chair, roundTable, toilet, basin, box, block,
} from './kit.js';

export const SOURCE = {
  title: 'Clinic',
  url: 'https://www.smartdraw.com/office-floor-plan/img/medical-office-layout.png',
  credit: 'SmartDraw physical therapy office sample',
  sizePx: [2108, 1300],
  pxPerMetre: 45,
  originPx: [13.5, 21],
};

export const WIDTH = 46.31;
export const DEPTH = 28;

// The double door in the south-west corner, into the waiting area.
export const ENTRANCE = [2.18, 26.18];

export { THICKNESS };

// Stepped outline: Offices 1–2 stand proud of the north facade, Offices 3–4 of
// the south one, and the gym's wet block steps down again at the south-east.
export const OUTLINE = [
  [0, 1.5], [17.7, 1.5], [17.7, 0], [26.19, 0],
  [26.19, 0.47], [46.31, 0.47], [46.31, 27.21], [38.96, 27.21],
  [38.96, 28], [31.03, 28], [31.03, 27.47], [26.67, 27.47],
  [26.67, 27.97], [17.7, 27.97], [17.7, 26.18], [0, 26.18],
];

const EXT_HALF = THICKNESS.exterior / 2;

// North and south runs reach past their corners by half a thickness so the
// corners close; east and west runs stop at the centreline inside them.
export const walls = [
  // exterior
  { id: 'north-west', kind: 'exterior', from: [0, 1.5], to: [17.7, 1.5], ext: EXT_HALF, openings: [win(3.12, 4.94), win(10.48, 12.26), win(14.46, 16.28)] },
  { id: 'office-west', kind: 'exterior', from: [17.7, 0], to: [17.7, 1.5], ext: 0, openings: [] },
  { id: 'office-north', kind: 'exterior', from: [17.7, 0], to: [26.19, 0], ext: EXT_HALF, openings: [win(18.88, 20.72), win(23.06, 24.92)] },
  { id: 'office-east', kind: 'exterior', from: [26.19, 0], to: [26.19, 0.47], ext: 0, openings: [] },
  { id: 'north-east', kind: 'exterior', from: [26.19, 0.47], to: [46.31, 0.47], ext: EXT_HALF, openings: [win(28.39, 30.26), win(34.48, 36.34)] },
  { id: 'east', kind: 'exterior', from: [46.31, 0.47], to: [46.31, 27.21], ext: 0, openings: [] },
  { id: 'gym-south', kind: 'exterior', from: [38.96, 27.21], to: [46.31, 27.21], ext: EXT_HALF, openings: [win(40.92, 44.1)] },
  { id: 'lobby-east', kind: 'exterior', from: [38.96, 27.21], to: [38.96, 28], ext: 0, openings: [] },
  { id: 'lobby-south', kind: 'exterior', from: [31.03, 28], to: [38.96, 28], ext: EXT_HALF, openings: [win(35.54, 36.61), door(37.39, 38.23, 'N', 'b')] },
  { id: 'step-east', kind: 'exterior', from: [31.03, 27.47], to: [31.03, 28], ext: 0, openings: [] },
  { id: 'supply-south', kind: 'exterior', from: [26.67, 27.47], to: [31.03, 27.47], ext: EXT_HALF, openings: [door(29.74, 30.59, 'N', 'b')] },
  { id: 'step-west', kind: 'exterior', from: [26.67, 27.47], to: [26.67, 27.97], ext: 0, openings: [] },
  { id: 'offices-south', kind: 'exterior', from: [17.7, 27.97], to: [26.67, 27.97], ext: EXT_HALF, openings: [win(19.19, 21.01), win(23.57, 25.39)] },
  { id: 'offices-west', kind: 'exterior', from: [17.7, 26.18], to: [17.7, 27.97], ext: 0, openings: [] },
  { id: 'south-west', kind: 'exterior', from: [0, 26.18], to: [17.7, 26.18], ext: EXT_HALF, openings: [doubleDoor(1.28, 3.08, 'N'), win(5.66, 7.48), win(10.23, 12.03), win(14.68, 16.48)] },
  { id: 'west', kind: 'exterior', from: [0, 1.5], to: [0, 26.18], ext: 0, openings: [] },

  // north row
  { id: 'restroom-nw-south', kind: 'interior', from: [0, 4.6], to: [2.64, 4.6], openings: [door(1.54, 2.39, 'N', 'b')] },
  { id: 'procedure1-west', kind: 'interior', from: [2.64, 1.5], to: [2.64, 6.14], openings: [] },
  { id: 'north-row-south', kind: 'interior', from: [2.64, 6.14], to: [17.7, 6.14], openings: [door(8.26, 9.1, 'N', 'b'), door(9.39, 10.23, 'N', 'a'), door(16.72, 17.54, 'N', 'b')] },
  { id: 'exam1-west', kind: 'interior', from: [9.24, 1.5], to: [9.24, 6.14], openings: [] },
  { id: 'exam1-east', kind: 'interior', from: [13.48, 1.5], to: [13.48, 6.14], openings: [] },
  { id: 'exam2-east', kind: 'interior', from: [17.7, 1.5], to: [17.7, 6.14], openings: [] },
  { id: 'offices-front', kind: 'interior', from: [17.7, 6.14], to: [26.19, 6.14], openings: [door(21.01, 21.88, 'N', 'b'), door(22.21, 23.08, 'N', 'a')] },
  { id: 'office-divider', kind: 'interior', from: [21.98, 0], to: [21.98, 6.14], openings: [] },
  { id: 'office2-east', kind: 'interior', from: [26.19, 0.47], to: [26.19, 6.14], openings: [] },

  // waiting, restroom and reception pod
  { id: 'restroom2-west', kind: 'interior', from: [2.97, 6.14], to: [2.97, 11.2], openings: [door(6.2, 7.11, 'E', 'b')] },
  { id: 'restroom2-south', kind: 'interior', from: [2.97, 11.2], to: [6.23, 11.2], openings: [] },
  { id: 'restroom2-east', kind: 'interior', from: [6.23, 6.14], to: [6.23, 11.2], openings: [] },
  { id: 'waiting-north', kind: 'interior', from: [6.23, 9.89], to: [9.2, 9.89], openings: [door(8.08, 8.94, 'S', 'b')] },
  { id: 'waiting-north-stub', kind: 'interior', from: [9.2, 9.89], to: [9.2, 10.63], openings: [] },
  { id: 'pod-north', kind: 'interior', from: [9.2, 10.63], to: [11.94, 10.63], openings: [] },
  { id: 'pod-west', kind: 'interior', from: [9.2, 10.63], to: [9.2, 16], openings: [win(10.76, 15.89, 1.05, 2.1)] },
  { id: 'pod-east', kind: 'interior', from: [11.94, 10.63], to: [11.94, 16], openings: [win(13.62, 15.58, 1.05, 2.1)] },
  { id: 'pod-south', kind: 'interior', from: [9.2, 16], to: [11.94, 16], openings: [] },
  { id: 'waiting-south-stub', kind: 'interior', from: [9.2, 16], to: [9.2, 16.91], openings: [] },
  { id: 'waiting-south', kind: 'interior', from: [5.98, 16.91], to: [9.2, 16.91], openings: [door(8.08, 8.94, 'N', 'b')] },
  { id: 'waiting-east', kind: 'interior', from: [5.98, 16.91], to: [5.98, 21.67], openings: [] },

  // south row
  { id: 'south-row-north', kind: 'interior', from: [4.5, 21.67], to: [26.67, 21.67], openings: [door(7.88, 8.7, 'S', 'b'), door(9.1, 9.97, 'S', 'a'), door(16.57, 17.41, 'S', 'b'), door(21.1, 21.94, 'S', 'b'), door(22.48, 23.32, 'S', 'a')] },
  { id: 'exam4-west', kind: 'interior', from: [4.5, 21.67], to: [4.5, 26.18], openings: [] },
  { id: 'exam4-east', kind: 'interior', from: [8.98, 21.67], to: [8.98, 26.18], openings: [] },
  { id: 'exam5-east', kind: 'interior', from: [13.44, 21.67], to: [13.44, 26.18], openings: [] },
  { id: 'exam6-east', kind: 'interior', from: [17.7, 21.67], to: [17.7, 26.18], openings: [] },
  { id: 'office-divider-south', kind: 'interior', from: [22.19, 21.67], to: [22.19, 27.97], openings: [] },
  { id: 'office4-east', kind: 'interior', from: [26.67, 21.67], to: [26.67, 27.47], openings: [] },
  { id: 'supply-north', kind: 'interior', from: [26.67, 23], to: [29.31, 23], openings: [door(27.46, 28.3, 'S', 'a')] },
  { id: 'supply-east', kind: 'interior', from: [29.31, 23], to: [29.31, 27.47], openings: [] },

  // centre block
  { id: 'block-west', kind: 'interior', from: [18.47, 9.9], to: [18.47, 16.77], openings: [door(12.04, 13.13, 'W', 'b')] },
  { id: 'block-north', kind: 'interior', from: [18.47, 9.9], to: [28.37, 9.9], openings: [door(19.74, 20.59, 'S', 'b'), opening(21.08, 22.03), door(25.48, 26.3, 'S', 'b')] },
  { id: 'block-south', kind: 'interior', from: [18.47, 16.77], to: [28.37, 16.77], openings: [door(19.74, 20.59, 'N', 'b'), opening(21.01, 21.94), door(24.86, 25.7, 'N', 'b'), door(27.08, 27.92, 'N', 'b')] },
  { id: 'restroom-a-south', kind: 'interior', from: [18.47, 12.01], to: [20.72, 12.01], openings: [] },
  { id: 'restroom-b-north', kind: 'interior', from: [18.47, 14.66], to: [20.72, 14.66], openings: [] },
  { id: 'lab-west-north', kind: 'interior', from: [20.72, 9.9], to: [20.72, 12.01], openings: [] },
  { id: 'lab-west-south', kind: 'interior', from: [20.72, 14.66], to: [20.72, 16.77], openings: [] },
  { id: 'anteroom-east', kind: 'interior', from: [20.19, 12.01], to: [20.19, 14.66], openings: [] },
  { id: 'lab-east', kind: 'interior', from: [24.14, 9.9], to: [24.14, 16.77], openings: [] },
  { id: 'restroom-c-south', kind: 'interior', from: [24.14, 11.59], to: [26.43, 11.59], openings: [] },
  { id: 'procedure2-east', kind: 'interior', from: [26.43, 9.9], to: [26.43, 16.77], openings: [] },
  { id: 'block-east', kind: 'interior', from: [28.37, 9.9], to: [28.37, 16.77], openings: [door(10.07, 10.93, 'W', 'a'), door(12.33, 13.18, 'E', 'b'), door(14.09, 14.93, 'E', 'b')] },
  { id: 'supply-c-south', kind: 'interior', from: [26.43, 11.67], to: [28.37, 11.67], openings: [] },
  { id: 'server1-south', kind: 'interior', from: [26.43, 13.36], to: [28.37, 13.36], openings: [] },
  { id: 'server2-south', kind: 'interior', from: [26.43, 15.07], to: [28.37, 15.07], openings: [] },

  // x-ray suite
  { id: 'xray-west', kind: 'interior', from: [33.47, 0.47], to: [33.47, 7.93], openings: [] },
  { id: 'control-south', kind: 'interior', from: [33.47, 3.36], to: [37.41, 3.36], openings: [win(33.97, 36.57, 0.9, 2.1)] },
  { id: 'control-east', kind: 'interior', from: [37.41, 0.47], to: [37.41, 3.36], openings: [door(1.16, 2.02, 'W', 'a')] },
  { id: 'xray-east', kind: 'interior', from: [38.74, 0.47], to: [38.74, 7.93], openings: [door(3.27, 4.11, 'W', 'a')] },
  { id: 'gym-north', kind: 'interior', from: [31.03, 7.93], to: [38.74, 7.93], openings: [] },

  // gym and its wet block
  { id: 'gym-west', kind: 'interior', from: [31.03, 7.93], to: [31.03, 27.47], openings: [doubleDoor(12.73, 14.58, 'E')] },
  { id: 'wet-north', kind: 'interior', from: [31.03, 20.58], to: [37.91, 20.58], openings: [door(32.88, 33.74, 'N', 'b'), door(34.1, 34.94, 'N', 'a')] },
  { id: 'restroom-up-south', kind: 'interior', from: [31.03, 24.67], to: [33.88, 24.67], openings: [] },
  { id: 'wet-divider', kind: 'interior', from: [33.88, 20.58], to: [33.88, 25.52], openings: [] },
  { id: 'wh-south', kind: 'interior', from: [33.88, 22.57], to: [35.21, 22.57], openings: [] },
  { id: 'wh-east', kind: 'interior', from: [35.21, 21.19], to: [35.21, 22.57], openings: [] },
  { id: 'showers-north', kind: 'interior', from: [35.21, 21.19], to: [37.91, 21.19], openings: [] },
  { id: 'showers-east', kind: 'interior', from: [37.91, 20.58], to: [37.91, 25.52], openings: [] },
  { id: 'showers-south', kind: 'interior', from: [33.88, 25.52], to: [37.91, 25.52], openings: [door(36.17, 37.21, 'S', 'b')] },
  { id: 'restroom-low-east', kind: 'interior', from: [34.89, 25.52], to: [34.89, 28], openings: [door(26.53, 27.36, 'W', 'b')] },
];

// zone drives the floor tint; floor drives the material (tile gets a grid).
export const rooms = [
  { id: 'restroom-nw', name: 'Restroom', zone: 'service', floor: 'tile', polygon: rect(0, 1.5, 2.64, 4.6) },
  { id: 'procedure-1', name: 'Procedure 1', zone: 'work', floor: 'tile', polygon: rect(2.64, 1.5, 9.24, 6.14) },
  { id: 'exam-1', name: 'Exam 1', zone: 'work', floor: 'tile', polygon: rect(9.24, 1.5, 13.48, 6.14) },
  { id: 'exam-2', name: 'Exam 2', zone: 'work', floor: 'tile', polygon: rect(13.48, 1.5, 17.7, 6.14) },
  { id: 'office-1', name: 'Office 1', zone: 'work', floor: 'carpet', polygon: rect(17.7, 0, 21.98, 6.14) },
  { id: 'office-2', name: 'Office 2', zone: 'work', floor: 'carpet', polygon: rect(21.98, 0, 26.19, 6.14) },
  { id: 'cafe', name: 'Café', zone: 'social', floor: 'wood', polygon: [[26.19, 0.47], [33.47, 0.47], [33.47, 7.93], [31.03, 7.93], [31.03, 6.14], [26.19, 6.14]], label: [29.26, 5.09] },
  { id: 'xray-control', name: 'X-Ray Control', zone: 'work', floor: 'tile', polygon: rect(33.47, 0.47, 37.41, 3.36) },
  { id: 'xray', name: 'X-Ray Room', zone: 'work', floor: 'tile', polygon: [[37.41, 0.47], [38.74, 0.47], [38.74, 7.93], [33.47, 7.93], [33.47, 3.36], [37.41, 3.36]], label: [36.14, 4.42] },
  { id: 'restroom-2', name: 'Restroom', zone: 'service', floor: 'tile', polygon: rect(2.97, 6.14, 6.23, 11.2) },
  { id: 'waiting', name: 'Waiting', zone: 'social', floor: 'carpet', polygon: [[0, 4.6], [2.64, 4.6], [2.64, 6.14], [2.97, 6.14], [2.97, 11.2], [6.23, 11.2], [6.23, 9.89], [9.2, 9.89], [9.2, 16.91], [5.98, 16.91], [5.98, 21.67], [4.5, 21.67], [4.5, 26.18], [0, 26.18]], label: [3.03, 18.64] },
  { id: 'reception', name: 'Reception', zone: 'work', floor: 'carpet', polygon: rect(9.2, 10.63, 11.94, 16) },
  { id: 'nurses', name: 'Nurses Station', zone: 'work', floor: 'tile', polygon: rect(13.7, 9.64, 18.47, 16.98), label: [16.14, 13.09] },
  { id: 'restroom-a', name: 'Restroom', zone: 'service', floor: 'tile', polygon: rect(18.47, 9.9, 20.72, 12.01) },
  { id: 'anteroom', name: 'Lab Prep', zone: 'work', floor: 'tile', polygon: rect(18.47, 12.01, 20.19, 14.66) },
  { id: 'restroom-b', name: 'Restroom', zone: 'service', floor: 'tile', polygon: rect(18.47, 14.66, 20.72, 16.77) },
  { id: 'lab', name: 'Lab Room', zone: 'work', floor: 'tile', polygon: [[20.72, 9.9], [24.14, 9.9], [24.14, 16.77], [20.72, 16.77], [20.72, 14.66], [20.19, 14.66], [20.19, 12.01], [20.72, 12.01]], label: [22.14, 13.31] },
  { id: 'restroom-c', name: 'Restroom', zone: 'service', floor: 'tile', polygon: rect(24.14, 9.9, 26.43, 11.59) },
  { id: 'procedure-2', name: 'Procedure 2', zone: 'work', floor: 'tile', polygon: rect(24.14, 11.59, 26.43, 16.77) },
  { id: 'supply-c', name: 'Supply', zone: 'service', floor: 'tile', polygon: rect(26.43, 9.9, 28.37, 11.67) },
  { id: 'server-1', name: 'Server 1', zone: 'service', floor: 'tile', polygon: rect(26.43, 11.67, 28.37, 13.36) },
  { id: 'server-2', name: 'Server 2', zone: 'service', floor: 'tile', polygon: rect(26.43, 13.36, 28.37, 15.07) },
  { id: 'janitor', name: 'Janitor', zone: 'service', floor: 'tile', polygon: rect(26.43, 15.07, 28.37, 16.77) },
  { id: 'exam-4', name: 'Exam 4', zone: 'work', floor: 'tile', polygon: rect(4.5, 21.67, 8.98, 26.18) },
  { id: 'exam-5', name: 'Exam 5', zone: 'work', floor: 'tile', polygon: rect(8.98, 21.67, 13.44, 26.18) },
  { id: 'exam-6', name: 'Exam 6', zone: 'work', floor: 'tile', polygon: rect(13.44, 21.67, 17.7, 26.18) },
  { id: 'office-3', name: 'Office 3', zone: 'work', floor: 'carpet', polygon: rect(17.7, 21.67, 22.19, 27.97) },
  { id: 'office-4', name: 'Office 4', zone: 'work', floor: 'carpet', polygon: rect(22.19, 21.67, 26.67, 27.97) },
  { id: 'supply-s', name: 'Supply', zone: 'service', floor: 'tile', polygon: rect(26.67, 23, 29.31, 27.47) },
  { id: 'corridor', name: 'Corridor', zone: 'service', floor: 'wood', polygon: [[6.23, 6.14], [31.03, 6.14], [31.03, 27.47], [29.31, 27.47], [29.31, 23], [26.67, 23], [26.67, 21.67], [5.98, 21.67], [5.98, 16.91], [9.2, 16.91], [9.2, 16], [11.94, 16], [11.94, 12.87], [13.7, 12.87], [13.7, 16.98], [18.47, 16.98], [18.47, 16.77], [28.37, 16.77], [28.37, 9.9], [18.47, 9.9], [18.47, 9.64], [13.7, 9.64], [13.7, 12.87], [11.94, 12.87], [11.94, 10.63], [9.2, 10.63], [9.2, 9.89], [6.23, 9.89]], label: [15.26, 7.98] },
  { id: 'gym', name: 'Gym', zone: 'social', floor: 'wood', polygon: [[38.74, 0.47], [46.31, 0.47], [46.31, 27.21], [38.96, 27.21], [38.96, 25.52], [37.91, 25.52], [37.91, 20.58], [31.03, 20.58], [31.03, 7.93], [38.74, 7.93]], label: [39.7, 9.53] },
  { id: 'restroom-up', name: 'Restroom', zone: 'service', floor: 'tile', polygon: rect(31.03, 20.58, 33.88, 24.67) },
  { id: 'restroom-low', name: 'Restroom', zone: 'service', floor: 'tile', polygon: [[31.03, 24.67], [33.88, 24.67], [33.88, 25.52], [34.89, 25.52], [34.89, 28], [31.03, 28]] },
  { id: 'water-heater', name: 'Water Heater', zone: 'service', floor: 'tile', polygon: [[33.88, 20.58], [37.91, 20.58], [37.91, 21.19], [35.21, 21.19], [35.21, 22.57], [33.88, 22.57]] },
  { id: 'showers', name: 'Showers', zone: 'service', floor: 'tile', polygon: [[35.21, 21.19], [37.91, 21.19], [37.91, 25.52], [33.88, 25.52], [33.88, 22.57], [35.21, 22.57]] },
  { id: 'vestibule', name: 'Vestibule', zone: 'service', floor: 'tile', polygon: [[34.89, 25.52], [38.96, 25.52], [38.96, 28], [34.89, 28]] },
];

export const floorPatches = [];

// ---- furniture, in source pixels
const P = fromPx(SOURCE);
const K = SOURCE.pxPerMetre;
const pbox = (type, x0, y0, x1, y1, h) => box(type, ...P(x0, y0), ...P(x1, y1), h);
const pblock = (x0, y0, x1, y1, h, mat, z0 = 0) => {
  const [a, b] = P(x0, y0);
  const [c, d] = P(x1, y1);
  return block((a + c) / 2, (b + d) / 2, +(c - a).toFixed(3), +(d - b).toFixed(3), h, mat, { z0 });
};
const at = (fn) => (px, py, ...rest) => fn(...P(px, py), ...rest);
const pchair = (px, py, facing, size = 0.55) => ({ ...chair(...P(px, py), facing), size });
const ptoilet = at(toilet);
const pbasin = at(basin);

// Exam tables are drawn at 99 × 35 px; a standard 1.9 × 0.7 m table sits centred on each.
const examTable = (px, py) => block(...P(px, py), 0.7, 1.9, 0.65, 'fabric');

// A counter with an inset sink bowl.
const sinkCounter = (x0, y0, x1, y1, sx0, sy0, sx1, sy1) => [
  pbox('counter', x0, y0, x1, y1, 0.9),
  pblock(sx0, sy0, sx1, sy1, 0.01, 'steel', 0.9),
];

// A square café table turned 45° with a chair off each edge.
function cafeTable(px, py) {
  const [x, y] = P(px, py);
  return [
    desk(x, y, 1.1, 1.1, 45),
    ...[45, 135, 225, 315].map((deg) => {
      const r = (deg * Math.PI) / 180;
      const cx = +(x + Math.cos(r) * 0.95).toFixed(3);
      const cy = +(y + Math.sin(r) * 0.95).toFixed(3);
      return chair(cx, cy, toward(cx, cy, x, y));
    }),
  ];
}

function roundTableFour(px, py) {
  const [x, y] = P(px, py);
  return [
    roundTable(x, y, 0.6),
    ...[[0, -1], [1, 0], [0, 1], [-1, 0]].map(([dx, dy]) => {
      const cx = +(x + dx * 0.88).toFixed(3);
      const cy = +(y + dy * 0.88).toFixed(3);
      return chair(cx, cy, toward(cx, cy, x, y));
    }),
  ];
}

// A treadmill along x: a low deck and a console at the end the runner faces.
function treadmill(x0, x1, cy, consoleEast) {
  const [a, y] = P(x0, cy);
  const [b] = P(x1, cy);
  const mid = (a + b) / 2;
  const cx = consoleEast ? mid + 0.95 : mid - 0.95;
  return [
    block(+mid.toFixed(3), y, 2.0, 0.85, 0.35, 'metal'),
    block(+cx.toFixed(3), y, 0.12, 0.85, 1.35, 'appliance'),
  ];
}

// Recumbent bike: frame, handlebar post at the west end, seat to the east.
const bike = (cy) => [
  pblock(2009, cy - 12, 2086, cy + 12, 0.4, 'metal'),
  pblock(2009, cy - 20, 2024, cy + 20, 1.1, 'metal'),
  pblock(2050, cy - 15, 2078, cy + 15, 0.3, 'fabric', 0.4),
];

// Flat bench with a barbell rack at its east end.
const benchPress = (cy) => [
  pblock(2009, cy - 11, 2068, cy + 11, 0.45, 'fabric'),
  pblock(2068, cy - 30, 2080, cy - 22, 1.2, 'metal'),
  pblock(2068, cy + 22, 2080, cy + 30, 1.2, 'metal'),
  pblock(2070, cy - 37, 2078, cy + 37, 0.04, 'steel', 1.1),
];

// Seated leg machine: frame, seat, and the weight stack upright at the east end.
const legMachine = (y0, y1) => [
  pblock(1527, y0, 1602, y1, 0.4, 'metal'),
  pblock(1540, y0 + 10, 1575, y1 - 10, 0.12, 'fabric', 0.4),
  pblock(1585, y0, 1602, y1, 1.7, 'metal'),
];

// Bench with an upright at mid-length.
const weightBench = (y0, y1) => [
  pblock(1525, (y0 + y1) / 2 - 9, 1600, (y0 + y1) / 2 + 9, 0.45, 'fabric'),
  pblock(1555, y0, 1565, y1, 1.2, 'metal'),
];

const shower = (x0, y0, x1, y1) => pblock(x0, y0, x1, y1, 0.08, 'ceramic');

// Waiting chairs are drawn at ~33 px centres, so they sit close together.
const row = (xs, py, facing) => xs.map((px) => pchair(px, py, facing));
const sideTable = (px, py) => pblock(px - 13, py - 13, px + 13, py + 13, 0.5, 'wood');

export const furniture = [
  // Restroom (north-west)
  ptoilet(50, 107, S), pbasin(104, 105, S),

  // Exam 1 and 2, with the shared sink counter straddling their party wall
  examTable(452, 174.5),
  ...sinkCounter(582, 93, 618, 205, 592, 110, 613, 136),
  ...sinkCounter(622, 93, 659, 205, 628, 110, 650, 136),
  examTable(786.5, 173),

  // Office 1
  pbox('desk', 814, 96, 852, 238, 0.75), pbox('desk', 852, 200, 952, 238, 0.75),
  pchair(866, 184, toward(866, 184, 835, 215), 0.55),
  pchair(864, 270, N), pchair(895, 270, N),
  at(roundTable)(959, 50, 0.42), pchair(931, 55, E),
  pbox('cabinet', 978, 112, 998, 173, 1.1),

  // Office 2
  pbox('desk', 1052, 96, 1090, 238, 0.75), pbox('desk', 1090, 200, 1188, 238, 0.75),
  pchair(1105, 184, toward(1105, 184, 1074, 215), 0.55),
  pchair(1103, 270, N), pchair(1134, 270, N),
  at(roundTable)(1163, 50, 0.42), pchair(1139, 55, E),
  pbox('cabinet', 1006, 31, 1026, 92, 1.1),

  // Café
  ...cafeTable(1263, 221), ...cafeTable(1390, 144),
  ...sinkCounter(1467, 48, 1516, 312, 1476, 206, 1505, 254),
  pblock(1465, 315, 1516, 373, 1.8, 'appliance'),

  // X-Ray control and room
  pbox('desk', 1525, 127, 1681, 168, 0.75), pchair(1590, 108, S),
  pblock(1544, 262, 1705, 323, 0.75, 'appliance'),
  pblock(1608, 327, 1735, 364, 1.0, 'appliance'),
  pblock(1625, 300, 1645, 325, 2.0, 'metal'),
  pblock(1617, 280, 1652, 305, 0.3, 'appliance', 1.3),

  // Restroom by the waiting area, with a changing table
  ptoilet(197, 507, N), pbasin(257, 510, N),
  pbox('cabinet', 213, 302, 262, 337, 0.9),

  // Waiting: chairs along the west wall with side tables between
  ...[512, 546, 606, 640, 920, 991, 1083, 1117].map((py) => pchair(33, py, E)),
  sideTable(33, 577), sideTable(33, 957), sideTable(33, 1182),
  ...row([170, 206, 238], 545, S),
  ...row([313, 347], 487, S),
  // two back-to-back rows in the middle
  ...row([113.5, 147.5, 180.5, 213.5, 247.5, 310.5], 660, N),
  ...row([113.5, 147.5, 180.5, 213.5, 247.5, 310.5], 696, S),
  sideTable(278.5, 660), sideTable(278.5, 696),
  pchair(300, 758, N), sideTable(332, 761),
  pchair(262, 936, W), pchair(234, 973, N), sideTable(265, 975),
  pchair(195, 1083, W), pchair(195, 1117, W),
  // planter box on the west wall
  pbox('cabinet', 19, 696, 59, 763, 0.7), pblock(22, 700, 56, 759, 0.35, 'plant', 0.7),

  // Reception pod: U-shaped counter facing the waiting-room glass
  pbox('counter', 433, 506, 464, 736, 1.0), pbox('counter', 464, 506, 531, 534, 1.0), pbox('counter', 464, 706, 531, 736, 1.0),
  pchair(515, 556, N, 0.5), pchair(481, 592, W, 0.5), pchair(481, 645, W, 0.5), pchair(515, 686, S, 0.5),

  // Nurses Station
  pbox('counter', 639, 463, 681, 776, 1.0), pbox('counter', 681, 463, 770, 504, 1.0), pbox('counter', 681, 733, 770, 776, 1.0),
  pchair(741, 522, N, 0.5), pchair(702, 566, W, 0.5), pchair(702, 619, W, 0.5), pchair(702, 672, W, 0.5), pchair(741, 715, S, 0.5),

  // Centre block restrooms
  ptoilet(863, 537, E), pbasin(856, 486, E),
  ptoilet(863, 705, E), pbasin(856, 753, E),
  ptoilet(1118, 525, E), pbasin(1112, 486, E),

  // Lab prep and Lab Room
  pbox('cabinet', 902, 571, 919, 622, 1.0), pblock(861, 637, 902, 677, 0.9, 'appliance'),
  pbox('counter', 925, 565, 957, 632, 0.9), pblock(926, 635, 966, 677, 0.9, 'appliance'),
  pblock(1061, 501, 1097, 550, 1.8, 'appliance'), pbox('counter', 1071, 552, 1097, 697, 0.9),

  // Exam 4, 5 and 6
  examTable(241, 1104.5),
  ...sinkCounter(379, 1085, 415, 1194, 388, 1157, 408, 1178),
  ...sinkCounter(420, 1085, 458, 1194, 431, 1157, 450, 1178),
  examTable(595, 1104.5),
  examTable(645, 1104.5),
  ...sinkCounter(771, 1085, 805, 1194, 781, 1155, 801, 1178),

  // Office 3 and 4: U-shaped desks with two visitor chairs
  ...[0, 202].flatMap((dx) => [
    pbox('desk', 830 + dx, 1109, 872 + dx, 1209, 0.75),
    pbox('desk', 872 + dx, 1109, 955 + dx, 1142, 0.75),
    pbox('desk', 955 + dx, 1109, 998 + dx, 1209, 0.75),
    pchair(913 + dx, 1180, N),
    pchair(894.5 + dx, 1065.5, S), pchair(928 + dx, 1065.5, S),
  ]),

  // Gym: lockers and cabinets along the walls
  pbox('cabinet', 1761, 48, 1835, 85, 0.9), pbox('cabinet', 1840, 48, 1914, 85, 0.9),
  pbox('cabinet', 1761, 214, 1784, 373, 1.8),
  pbox('cabinet', 1723, 954, 1748, 1035, 1.8), pbox('cabinet', 1723, 1040, 1748, 1119, 1.8),
  ...roundTableFour(1898, 177), ...roundTableFour(2013, 228), ...roundTableFour(1909, 312),
  ...legMachine(498, 546), ...legMachine(586, 635),
  ...weightBench(671, 722), ...weightBench(752, 805),
  // treadmills: the west column faces west, the east column faces east
  ...[536.5, 618, 710, 808].flatMap((cy) => [...treadmill(1703, 1798, cy, false), ...treadmill(1830, 1925, cy, true)]),
  ...bike(403), ...bike(473), ...bike(551),
  pblock(2050, 646, 2086, 722, 0.9, 'metal'), pblock(2050, 738, 2086, 813, 0.9, 'metal'),
  ...benchPress(1002), ...benchPress(1095.5), ...benchPress(1184),

  // Wet block
  ptoilet(1433, 1113, N), pbasin(1486, 1118, N),
  ptoilet(1435, 1149, S), pbasin(1488, 1150, S),
  pblock(1552, 993, 1584, 1025, 1.5, 'appliance'),
  shower(1602, 978, 1652, 1029), shower(1668, 978, 1716, 1029),
  shower(1542, 1040, 1594, 1091), shower(1542, 1115, 1594, 1165),
];
