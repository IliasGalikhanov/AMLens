import { createGraphSimulation, type ForceSettings, type LinkInput, type NodeInput, type Point } from './graphPhysics';

export interface LayoutInput {
  nodes: NodeInput[];
  edges: LinkInput[];
  previous: [string, Point][];
  forces: ForceSettings;
  motion: boolean;
}
export interface LayoutFrame {
  ids?: string[];
  positions: Float32Array;
  settled: boolean;
}
export type LayoutCommand = { type: 'init'; input: LayoutInput }
  | { type: 'configure'; forces: ForceSettings; motion: boolean }
  | { type: 'pin'; id: string; point: Point | null };

/** Every batch yields, including reduced-motion rendering and the main-thread fallback. */
export function createLayoutTask(
  input: LayoutInput,
  publish: (frame: LayoutFrame) => void,
  schedule: (callback: () => void) => () => void = (callback) => {
    const handle = setTimeout(callback, input.nodes.length > 250 ? 0 : 16);
    return () => clearTimeout(handle);
  },
) {
  const simulation = createGraphSimulation(input.nodes, input.edges, new Map(input.previous));
  simulation.configure(input.forces);
  let motion = input.motion;
  let cancelled = false;
  let scheduled = false;
  let cancelPending: (() => void) | undefined;
  let steps = 0;
  let lastPublish = 0;
  const snapshot = (settled: boolean, includeIds = false) => {
    const positions = new Float32Array(simulation.nodes.length * 2);
    for (let i = 0; i < simulation.nodes.length; i += 1) {
      positions[i * 2] = simulation.nodes[i].x; positions[i * 2 + 1] = simulation.nodes[i].y;
    }
    publish({ positions, settled, ...(includeIds ? { ids: simulation.nodes.map((node) => node.id) } : {}) });
    lastPublish = performance.now();
  };
  const queue = () => {
    if (cancelled || scheduled) return;
    scheduled = true;
    cancelPending = schedule(tick);
  };
  const tick = () => {
    scheduled = false;
    if (cancelled) return;
    const started = performance.now();
    let moving = true;
    do { moving = simulation.step(); steps += 1; }
    while (moving && steps < 450 && input.nodes.length > 250 && performance.now() - started < 7);
    const settled = !moving || steps >= 450;
    // Large canvases redraw at most 10 fps while finding their layout.
    const interval = simulation.nodes.length > 5000 ? 200 : simulation.nodes.length > 250 ? 100 : 30;
    if (settled || (motion && performance.now() - lastPublish >= interval)) snapshot(settled);
    if (!settled) queue();
  };
  snapshot(false, true);
  queue();
  return {
    configure(forces: ForceSettings, nextMotion: boolean) {
      if (cancelled) return;
      motion = nextMotion; steps = 0; simulation.configure(forces); queue();
    },
    pin(id: string, point: Point | null) {
      if (cancelled) return;
      steps = 0; simulation.pin(id, point); queue();
    },
    dispose() { cancelled = true; cancelPending?.(); },
  };
}
