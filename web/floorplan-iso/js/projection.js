// Parallel projection from plan metres (x right, y down, z up) to screen metres.
//
// The camera orbits the rotation centre: azimuth θ turns the plan about the
// vertical axis, then elevation φ tilts it toward the viewer. At φ = 90° the
// result is the plan seen from directly above, exactly as drawn in the source.
// At θ = ±45° and φ = atan(1/√2) the three axes foreshorten equally, which is
// true isometric. Tweening θ and φ morphs smoothly between the two.

const DEG = Math.PI / 180;

export const ISO_ELEVATION = Math.atan(Math.SQRT1_2) / DEG; // 35.264°

export const PRESETS = {
  iso: { theta: -45, phi: ISO_ELEVATION },
  bird: { theta: -30, phi: 55 },
  plan: { theta: 0, phi: 90 },
};

export function makeView({ theta, phi, cx = 0, cy = 0 }) {
  const ct = Math.cos(theta * DEG);
  const st = Math.sin(theta * DEG);
  const sp = Math.sin(phi * DEG);
  const cp = Math.cos(phi * DEG);

  return {
    theta,
    phi,
    cx,
    cy,
    // Unit vector from the scene toward the camera, in plan axes. Its horizontal
    // part decides which side faces are visible and which way the depth order runs.
    toViewer: [cp * st, cp * ct, sp],

    // Plan point → rotated plan point. yr grows toward the camera.
    rotate(x, y) {
      const dx = x - cx;
      const dy = y - cy;
      return [dx * ct - dy * st, dx * st + dy * ct];
    },

    // Returns [sx, sy, depth]; larger depth is nearer the camera.
    project(x, y, z) {
      const dx = x - cx;
      const dy = y - cy;
      const xr = dx * ct - dy * st;
      const yr = dx * st + dy * ct;
      return [xr, yr * sp - z * cp, yr * cp + z * sp];
    },

    // Plan-space normal → its x component after rotation; used for shading so the
    // light stays fixed relative to the viewer while the model turns.
    screenNormalX(nx, ny) {
      return nx * ct - ny * st;
    },
  };
}
