import { createLayoutTask, type LayoutCommand, type LayoutFrame, type LayoutInput } from './graphLayoutTask';
import type { ForceSettings, Point } from './graphPhysics';

/** Worker owns the simulation. Termination cancels initialization as well as later batches. */
export function startGraphLayout(input: LayoutInput, publish: (frame: LayoutFrame) => void) {
  let worker: Worker | undefined;
  let fallback: ReturnType<typeof createLayoutTask> | undefined;
  let disposed = false;
  const pins = new Map<string, Point>();
  const send = (message: LayoutCommand) => worker?.postMessage(message);
  const startFallback = () => {
    worker?.terminate(); worker = undefined;
    if (!disposed) {
      fallback = createLayoutTask(input, publish);
      for (const [id, point] of pins) fallback.pin(id, point);
    }
  };
  if (input.nodes.length > 250 && typeof Worker !== 'undefined') {
    try {
      worker = new Worker(new URL('./graphLayout.worker.ts', import.meta.url), { type: 'module' });
      worker.onmessage = (event: MessageEvent<LayoutFrame>) => { if (!disposed) publish(event.data); };
      worker.onerror = (event) => { event.preventDefault(); startFallback(); };
      send({ type: 'init', input });
    } catch { startFallback(); }
  } else startFallback();
  return {
    configure(forces: ForceSettings, motion: boolean) {
      if (disposed) return;
      input.forces = forces; input.motion = motion;
      send({ type: 'configure', forces, motion }); fallback?.configure(forces, motion);
    },
    pin(id: string, point: Point | null) {
      if (disposed) return;
      if (point) pins.set(id, point); else pins.delete(id);
      send({ type: 'pin', id, point }); fallback?.pin(id, point);
    },
    dispose() { disposed = true; worker?.terminate(); fallback?.dispose(); },
  };
}
