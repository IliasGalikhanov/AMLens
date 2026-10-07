export interface ForceSettings {
  repulsion: number;
  distance: number;
  center: number;
}

export const defaultForces: ForceSettings = { repulsion: 1, distance: 1, center: 1 };
export interface Point { x: number; y: number }
export interface NodeInput { id: string; radius: number; cluster: number }
export interface LinkInput { source: string; target: string }
export interface MovingNode extends NodeInput, Point {
  vx: number;
  vy: number;
  fixed: boolean;
}

interface Quad {
  x: number; y: number; size: number; cx: number; cy: number; count: number; radius: number;
  children?: Quad[]; points?: number[];
}

function buildTree(nodes: MovingNode[]): Quad {
  let minX = Infinity; let minY = Infinity; let maxX = -Infinity; let maxY = -Infinity;
  for (const node of nodes) {
    minX = Math.min(minX, node.x); minY = Math.min(minY, node.y);
    maxX = Math.max(maxX, node.x); maxY = Math.max(maxY, node.y);
  }
  const build = (indices: number[], x: number, y: number, size: number, depth: number): Quad => {
    let cx = 0; let cy = 0; let radius = 0;
    for (const i of indices) { cx += nodes[i].x; cy += nodes[i].y; radius = Math.max(radius, nodes[i].radius); }
    const cell: Quad = { x, y, size, cx: cx / indices.length, cy: cy / indices.length, count: indices.length, radius };
    if (indices.length <= 4 || depth === 20 || size < 0.01) { cell.points = indices; return cell; }
    const half = size / 2;
    const groups: number[][] = [[], [], [], []];
    for (const i of indices) groups[(nodes[i].x >= x + half ? 1 : 0) + (nodes[i].y >= y + half ? 2 : 0)].push(i);
    cell.children = groups.flatMap((group, i) => group.length
      ? [build(group, x + (i % 2) * half, y + Math.floor(i / 2) * half, half, depth + 1)] : []);
    return cell;
  };
  return build(nodes.map((_, i) => i), minX, minY, Math.max(1, maxX - minX, maxY - minY) + 0.01, 0);
}

/** Exact repulsion for local slices, Barnes–Hut for complete networks; no node or edge sampling. */
export function createGraphSimulation(
  inputs: NodeInput[],
  edges: LinkInput[],
  previous: ReadonlyMap<string, Point> = new Map(),
) {
  const sorted = [...inputs].sort((a, b) => a.id.localeCompare(b.id));
  const clusters = [...new Set(sorted.map((node) => node.cluster))].sort((a, b) => a - b);
  const clusterIndices = new Map(clusters.map((cluster, i) => [cluster, i]));
  const counts = new Map<number, number>();
  const nodes: MovingNode[] = sorted.map((node) => {
    const index = counts.get(node.cluster) ?? 0;
    counts.set(node.cluster, index + 1);
    const group = clusterIndices.get(node.cluster)!;
    const groupAngle = clusters.length > 12 ? group * 2.399963229728653 : group * Math.PI * 2 / Math.max(1, clusters.length);
    const groupRadius = clusters.length > 1 ? 65 * Math.sqrt(inputs.length / clusters.length)
      * (clusters.length > 12 ? Math.sqrt(group + 1) : 1) : 0;
    const angle = index * 2.399963229728653;
    const radius = 22 * Math.sqrt(index + 1);
    const stored = previous.get(node.id);
    const point = stored && Number.isFinite(stored.x) && Number.isFinite(stored.y) ? stored : {
      x: Math.cos(groupAngle) * groupRadius + Math.cos(angle) * radius,
      y: Math.sin(groupAngle) * groupRadius + Math.sin(angle) * radius,
    };
    return { ...node, ...point, vx: 0, vy: 0, fixed: false };
  });
  const byId = new Map(nodes.map((node) => [node.id, node]));
  const degree = new Map(nodes.map((node) => [node.id, 0]));
  const seen = new Set<string>();
  // Reciprocal payments remain distinct in the renderer, but share one spring.
  const links = edges.flatMap((edge) => {
    const source = byId.get(edge.source);
    const target = byId.get(edge.target);
    const key = JSON.stringify([edge.source, edge.target].sort());
    if (!source || !target || source === target || seen.has(key)) return [];
    seen.add(key);
    degree.set(source.id, degree.get(source.id)! + 1);
    degree.set(target.id, degree.get(target.id)! + 1);
    return [{ source, target }];
  }).sort((a, b) => a.source.id.localeCompare(b.source.id) || a.target.id.localeCompare(b.target.id));
  let alpha = 1;
  let settings = { ...defaultForces };
  let repulsionVisits = 0;

  function wake() { alpha = Math.max(alpha, 0.5); }
  function pin(id: string, point: Point | null) {
    const node = byId.get(id);
    if (!node) return;
    if (point && (!Number.isFinite(point.x) || !Number.isFinite(point.y))) return;
    node.fixed = point !== null;
    if (point) { node.x = point.x; node.y = point.y; }
    node.vx = 0; node.vy = 0;
    wake();
  }
  function configure(next: ForceSettings) {
    const clamp = (value: number) => Number.isFinite(value) ? Math.max(0.25, Math.min(3, value)) : 1;
    settings = { repulsion: clamp(next.repulsion), distance: clamp(next.distance), center: clamp(next.center) };
    wake();
  }
  function step(): boolean {
    if (!nodes.length) return false;
    repulsionVisits = 0;
    const applyRepulsion = (a: MovingNode, x: number, y: number, mass: number, radius: number, seed: number, collision: boolean) => {
      repulsionVisits += 1;
      let dx = x - a.x; let dy = y - a.y;
      if (Math.abs(dx) + Math.abs(dy) < 0.01) {
        dx = Math.cos(seed * 2.399963229728653) * 0.1;
        dy = Math.sin(seed * 2.399963229728653) * 0.1;
      }
      const distance = Math.hypot(dx, dy);
      const repel = 1700 * settings.repulsion * mass / Math.max(distance * distance, 80);
      const overlap = collision ? Math.max(0, a.radius + radius + 12 - distance) * 0.18 : 0;
      const force = (repel * alpha + overlap) / distance;
      a.vx -= dx * force; a.vy -= dy * force;
    };
    if (nodes.length > 160) {
      const tree = buildTree(nodes);
      for (let i = 0; i < nodes.length; i += 1) {
        const a = nodes[i];
        const visit = (cell: Quad) => {
          const dx = cell.cx - a.x; const dy = cell.cy - a.y;
          const distanceSquared = dx * dx + dy * dy;
          const contains = a.x >= cell.x && a.x <= cell.x + cell.size && a.y >= cell.y && a.y <= cell.y + cell.size;
          if (!contains && cell.size * cell.size < 0.64 * distanceSquared) {
            applyRepulsion(a, cell.cx, cell.cy, cell.count, cell.radius, i + 1, true);
          } else if (cell.children) {
            for (const child of cell.children) visit(child);
          } else if (cell.points!.length > 4) {
            // A degenerate leaf may contain thousands of identical coordinates.
            // Aggregate it instead of falling back to an unbounded all-pairs loop.
            const mass = cell.count - (contains ? 1 : 0);
            if (mass) applyRepulsion(a, cell.cx, cell.cy, mass, cell.radius, i + 1, true);
          } else {
            for (const j of cell.points!) if (j !== i) applyRepulsion(a, nodes[j].x, nodes[j].y, 1, nodes[j].radius, i + j + 1, true);
          }
        };
        visit(tree);
      }
    } else for (let i = 0; i < nodes.length; i += 1) {
      const a = nodes[i];
      for (let j = i + 1; j < nodes.length; j += 1) {
        repulsionVisits += 1;
        const b = nodes[j];
        let dx = b.x - a.x; let dy = b.y - a.y;
        if (Math.abs(dx) + Math.abs(dy) < 0.01) {
          const angle = (i + j + 1) * 2.399963229728653;
          dx = Math.cos(angle) * 0.1; dy = Math.sin(angle) * 0.1;
        }
        const distance = Math.hypot(dx, dy);
        const spacing = a.radius + b.radius + 12;
        const repel = 1700 * settings.repulsion / Math.max(distance * distance, 80);
        const collision = Math.max(0, spacing - distance) * 0.18;
        const force = (repel * alpha + collision) / distance;
        a.vx -= dx * force; a.vy -= dy * force;
        b.vx += dx * force; b.vy += dy * force;
      }
    }
    for (const { source, target } of links) {
      const dx = target.x - source.x; const dy = target.y - source.y;
      const distance = Math.max(0.1, Math.hypot(dx, dy));
      const sameCluster = source.cluster === target.cluster;
      const rest = (sameCluster ? 58 : 110) * settings.distance
        + Math.sqrt(Math.max(degree.get(source.id)!, degree.get(target.id)!)) * 4;
      const strength = 0.055 / Math.sqrt(Math.min(degree.get(source.id)!, degree.get(target.id)!));
      const force = (distance - rest) * strength * alpha / distance;
      const bias = degree.get(source.id)! / (degree.get(source.id)! + degree.get(target.id)!);
      source.vx += dx * force * (1 - bias); source.vy += dy * force * (1 - bias);
      target.vx -= dx * force * bias; target.vy -= dy * force * bias;
    }
    let moving = false;
    for (const node of nodes) {
      if (node.fixed) { node.vx = 0; node.vy = 0; continue; }
      const gravity = 0.002 * settings.center * alpha / Math.max(1, Math.sqrt(nodes.length / 160));
      node.vx = (node.vx - node.x * gravity) * 0.72;
      node.vy = (node.vy - node.y * gravity) * 0.72;
      const speed = Math.hypot(node.vx, node.vy);
      if (speed > 8) { node.vx *= 8 / speed; node.vy *= 8 / speed; }
      node.x += node.vx; node.y += node.vy;
      if (speed > 0.025) moving = true;
    }
    alpha *= 0.97;
    return alpha > 0.008 || moving;
  }
  return { nodes, step, wake, pin, configure, statistics: () => ({ repulsionVisits, springs: links.length }) };
}

export type GraphSimulation = ReturnType<typeof createGraphSimulation>;
