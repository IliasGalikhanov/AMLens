import { t, getLocale } from '../../i18n/core.ts';
import { useLocale } from '../../i18n/react';
import { useEffect, useId, useRef, useState } from 'react';
import cytoscape, { type Core, type StylesheetJson } from 'cytoscape';
import type { GraphViewProps, Role } from '../../shared/contracts';
import { graphElements } from './graphModel';
import { planGraphLabels, type GraphLabel } from './graphLabels';
import { defaultForces, type ForceSettings, type Point } from './graphPhysics';
import { startGraphLayout } from './graphLayout';
import type { LayoutFrame } from './graphLayoutTask';
import { createGraphFocus, supportsGraphWebGL } from './graphRendering';
import './GraphView.css';

const roles: { role: Role; label: string }[] = [
  { role: 'consolidator', get label() { return t("Консолидация"); } },
  { role: 'transit', get label() { return t("Транзит"); } },
  { role: 'distributor', get label() { return t("Распределение"); } },
  { role: 'terminal', get label() { return t("Конечный получатель"); } },
  { role: 'coordinator', get label() { return t("Координация"); } },
  { role: 'peripheral', get label() { return t("Периферия"); } },
];

function fitGraph(cy: Core, global = false, padding = 38) {
  cy.resize();
  let bounds = cy.nodes().boundingBox({ includeLabels: false, includeOverlays: false, includeUnderlays: false });
  // Fit model geometry; the overview's minimum pixel sizes must not feed back into its zoom.
  if (global) {
    bounds = { x1: Infinity, y1: Infinity, x2: -Infinity, y2: -Infinity, w: 0, h: 0 };
    cy.nodes().forEach((node) => {
      const position = node.position();
      const radius = node.data('diameter') / 2;
      bounds.x1 = Math.min(bounds.x1, position.x - radius); bounds.x2 = Math.max(bounds.x2, position.x + radius);
      bounds.y1 = Math.min(bounds.y1, position.y - radius); bounds.y2 = Math.max(bounds.y2, position.y + radius);
    });
    if (!Number.isFinite(bounds.x1)) return;
    bounds.w = bounds.x2 - bounds.x1; bounds.h = bounds.y2 - bounds.y1;
  }
  const inset = Math.min(padding, cy.height() / 8);
  const zoom = Math.max(cy.minZoom(), Math.min(1.4,
    (cy.width() - 2 * inset) / Math.max(bounds.w, 1),
    (cy.height() - 2 * inset) / Math.max(bounds.h, 1)));
  cy.viewport({ zoom, pan: {
    x: cy.width() / 2 - zoom * (bounds.x1 + bounds.x2) / 2,
    y: cy.height() / 2 - zoom * (bounds.y1 + bounds.y2) / 2,
  } });
}

function SettingsIcon() {
  useLocale();
  return <svg width="16" height="16" viewBox="0 0 20 20" fill="none" aria-hidden="true">
    <path d="M3 5h14M3 10h14M3 15h14" stroke="currentColor" strokeWidth="1.3" strokeLinecap="round" />
    <circle cx="7" cy="5" r="2" fill="var(--color-surface)" stroke="currentColor" strokeWidth="1.3" />
    <circle cx="13" cy="10" r="2" fill="var(--color-surface)" stroke="currentColor" strokeWidth="1.3" />
    <circle cx="8" cy="15" r="2" fill="var(--color-surface)" stroke="currentColor" strokeWidth="1.3" />
  </svg>;
}

export default function GraphView({ graph, selectedGid, loading, onSelectGid }: GraphViewProps) {
  useLocale();
  const containerRef = useRef<HTMLDivElement>(null);
  const cyRef = useRef<Core | null>(null);
  const [labels, setLabels] = useState<GraphLabel[]>([]);
  const [showContext, setShowContext] = useState(true);
  const [zoomPercent, setZoomPercent] = useState(100);
  const [settingsOpen, setSettingsOpen] = useState(false);
  const [layoutBusy, setLayoutBusy] = useState(false);
  const [motion, setMotion] = useState(() => !window.matchMedia('(prefers-reduced-motion: reduce)').matches);
  const [forces, setForces] = useState<ForceSettings>({ ...defaultForces });
  const settingsId = useId();
  const optionsRef = useRef({ showContext, motion, forces });
  optionsRef.current = { showContext, motion, forces };
  const refreshLabelsRef = useRef<(() => void) | null>(null);
  const updateForcesRef = useRef<(() => void) | null>(null);
  const updateFocusRef = useRef<(() => void) | null>(null);
  const positionsRef = useRef(new Map<string, Point>());
  const selectRef = useRef(onSelectGid);
  selectRef.current = onSelectGid;
  const selectionRef = useRef(selectedGid);
  selectionRef.current = selectedGid;
  const hasNodes = !loading && graph !== null && graph.nodes.length > 0;
  const isGlobal = graph?.center_gid === '';
  const minZoom = isGlobal ? 0.001 : 0.08;
  const selectedMissing = hasNodes && selectedGid !== null && !graph.nodes.some((node) => node.gid === selectedGid);

  useEffect(() => {
    const container = containerRef.current;
    if (!hasNodes || !container || !graph) return;
    const root = getComputedStyle(document.documentElement);
    const token = (name: string) => root.getPropertyValue(name).trim();
    const number = (name: string) => Number.parseFloat(token(name));
    const reducedMotion = window.matchMedia('(prefers-reduced-motion: reduce)');
    const style: StylesheetJson = [
      { selector: 'node', style: {
        shape: 'ellipse', width: 'data(diameter)', height: 'data(diameter)',
        'background-color': token('--role-peripheral'), opacity: number('--graph-node-opacity'),
        'border-color': token('--graph-node-border'), 'border-width': number('--graph-node-border-width'),
        label: '', 'overlay-opacity': 0,
      } },
      ...roles.map(({ role }) => ({ selector: 'node[role = "' + role + '"]', style: { 'background-color': token('--role-' + role) } })),
      { selector: 'node.is-seed', style: {
        'border-width': number('--graph-seed-border-width'), 'border-color': token('--graph-seed-border-color'),
      } },
      { selector: 'node.is-neighbor', style: {
        'border-width': number('--graph-neighbor-border-width'), 'border-color': token('--graph-neighbor-color'),
      } },
      { selector: 'node.is-selected', style: {
        'border-width': number('--graph-selected-border-width'), 'border-color': token('--graph-selected-color'),
        'underlay-color': token('--graph-selected-halo-color'), 'underlay-opacity': number('--graph-selected-halo-opacity'),
        'underlay-padding': number('--graph-selected-halo-padding'), 'underlay-shape': 'ellipse',
      } },
      { selector: 'node.is-hovered', style: {
        'border-width': 2, 'border-color': token('--graph-selected-color'),
        'underlay-color': token('--graph-selected-halo-color'), 'underlay-opacity': 0.12,
        'underlay-padding': 4, 'underlay-shape': 'ellipse',
      } },
      { selector: 'node.is-dimmed', style: { opacity: number('--graph-dimmed-node-opacity') } },
      { selector: 'edge', style: {
        width: 'data(width)', 'line-color': token('--graph-edge-color'), opacity: number('--graph-edge-opacity'),
        'target-arrow-color': token('--graph-arrow-color'), 'target-arrow-shape': 'triangle',
        'arrow-scale': number('--graph-arrow-scale'), 'curve-style': 'bezier', 'overlay-opacity': 0,
      } },
      { selector: 'edge.is-path', style: {
        'line-color': token('--graph-path-color'), 'target-arrow-color': token('--graph-path-color'),
        opacity: graph.edges.length > 40 ? 0.7 : number('--graph-path-opacity'),
      } },
      { selector: 'edge.is-dimmed', style: { opacity: number('--graph-dimmed-edge-opacity') } },
    ];
    const elements = graphElements(graph, {
        minNode: number('--graph-node-min-size'), maxNode: number('--graph-node-max-size'),
        minEdge: number('--graph-edge-min-width'), maxEdge: number('--graph-edge-max-width'),
        defaultEdge: number('--graph-edge-default-width'),
      });
    container.style.backgroundColor = token('--color-graph-bg');
    const cy = cytoscape({
      container, elements: [],
      webgl: supportsGraphWebGL(),
      style, minZoom: graph.center_gid === '' ? 0.001 : 0.08, maxZoom: 4, boxSelectionEnabled: false,
      autounselectify: true, layout: { name: 'preset' },
      pixelRatio: graph.nodes.length > 2500 ? 1 : 'auto',
    });
    cyRef.current = cy;
    const gpuContext = container.querySelector<HTMLCanvasElement>('canvas[data-id="layer3-webgl"]')?.getContext('webgl2');
    setLayoutBusy(true);
    let layout: ReturnType<typeof startGraphLayout> | undefined;
    let disposed = false;
    let addFrame = 0;
    let labelFrame = 0;
    let renderFrame = 0;
    let resizeFrame = 0;
    let styleTimer = 0;
    let hoveredGid: string | null = null;
    let strokeScale = 1;
    let overviewZoom = 0;
    let grabbedGid: string | null = null;
    let ids: string[] = [];
    let latestFrame: LayoutFrame | null = null;
    let renderingFrame: LayoutFrame | null = null;
    let renderIndex = 0;
    let initialFit = false;
    let userViewport = false;
    let fitting = false;
    const priorities = new Map(graph.nodes.map((node) => [node.gid, node.priority_score]));
    const applyFocus = createGraphFocus(cy);
    const fit = () => { fitting = true; fitGraph(cy, graph.center_gid === ''); fitting = false; updateLabels(); };
    const renderPositions = () => {
      renderFrame = 0;
      if (disposed || !latestFrame) return;
      if (!renderingFrame) { renderingFrame = latestFrame; renderIndex = 0; }
      const current = renderingFrame;
      const started = performance.now();
      cy.batch(() => {
        let added = 0;
        while (renderIndex < ids.length && added < 500 && performance.now() - started < 8) {
          const id = ids[renderIndex];
          if (id !== grabbedGid) {
            const node = cy.getElementById(id);
            const x = current.positions[renderIndex * 2], y = current.positions[renderIndex * 2 + 1];
            const previous = node.position();
            // Ignore subpixel motion while settling, but always apply the final exact positions.
            if (current.settled || !initialFit || Math.hypot(x - previous.x, y - previous.y) * cy.zoom() >= 0.25) node.position({ x, y });
          }
          renderIndex += 1; added += 1;
        }
      });
      if (renderIndex === ids.length) {
        renderingFrame = null;
        if (!initialFit || (current.settled && !userViewport)) { fit(); initialFit = true; }
        if (current.settled) setLayoutBusy(false);
        updateLabels();
      }
      if (renderingFrame || latestFrame !== current) renderFrame = requestAnimationFrame(renderPositions);
    };
    const updateViewportStyles = () => {
        if (disposed || cy.destroyed()) return;
        const nextScale = Math.max(1, cy.zoom());
        const nextZoom = cy.zoom();
        const overviewChanged = graph.center_gid === '' && nextZoom !== overviewZoom;
        if (nextScale !== strokeScale || overviewChanged) {
          strokeScale = nextScale;
          overviewZoom = nextZoom;
          cy.batch(() => {
            if (overviewChanged) cy.nodes().forEach((node) => {
              const diameter = Math.max(node.data('diameter'), 2.8 / nextZoom);
              if (node.width() !== diameter) node.style({ width: diameter, height: diameter });
            });
            cy.edges().forEach((edge) => {
              const width = edge.data('width') / strokeScale;
              const nextWidth = graph.center_gid === '' ? Math.max(width, 0.32 / nextZoom) : width;
              const arrowScale = number('--graph-arrow-scale') / strokeScale;
              if (edge.numericStyle('width') !== nextWidth || edge.numericStyle('arrow-scale') !== arrowScale) {
                edge.style({ width: nextWidth, 'arrow-scale': arrowScale });
              }
            });
          });
        }
    };
    const scheduleViewportStyles = () => {
      window.clearTimeout(styleTimer);
      // During wheel/pinch gestures only the viewport transforms; restyle once they stop.
      styleTimer = window.setTimeout(() => { updateViewportStyles(); updateLabels(); }, 120);
    };
    const updateLabels = () => {
      if (labelFrame) return;
      labelFrame = requestAnimationFrame(() => {
        labelFrame = 0;
        if (disposed || cy.destroyed() || !initialFit) return;
        const showLabels = optionsRef.current.showContext && cy.zoom() >= 0.38;
        const nextLabels = !showLabels && !selectionRef.current && !hoveredGid ? [] : planGraphLabels(cy.nodes().map((node) => {
          const point = node.renderedPosition();
          return { gid: node.id(), x: point.x, y: point.y, radius: node.renderedWidth() / 2 + 3,
            priority: priorities.get(node.id()) ?? 0 };
        }), cy.width(), cy.height(), { selectedGid: selectionRef.current, hoveredGid,
          showContext: showLabels });
        setLabels((previous) => JSON.stringify(previous) === JSON.stringify(nextLabels) ? previous : nextLabels);
        setZoomPercent(Math.round(cy.zoom() * 1000) / 10);
      });
    };
    const updateFocus = () => { applyFocus(selectionRef.current, hoveredGid); updateLabels(); };
    refreshLabelsRef.current = updateLabels;
    updateFocusRef.current = updateFocus;
    const canAnimate = () => optionsRef.current.motion && !reducedMotion.matches;
    const resume = () => {
      layout?.configure(optionsRef.current.forces, canAnimate());
      updateLabels();
    };
    updateForcesRef.current = resume;
    cy.on('tap', 'node', (event) => selectRef.current(event.target.id()));
    cy.on('mouseover', 'node', (event) => {
      hoveredGid = event.target.id(); container.style.cursor = 'grab'; updateFocus();
    });
    cy.on('mouseout', 'node', () => {
      if (grabbedGid) return;
      hoveredGid = null; container.style.cursor = ''; updateFocus();
    });
    cy.on('grab', 'node', (event) => {
      grabbedGid = event.target.id(); hoveredGid = grabbedGid;
      layout?.pin(event.target.id(), event.target.position());
      userViewport = true;
      container.style.cursor = 'grabbing'; updateFocus();
    });
    cy.on('drag', 'node', (event) => {
      layout?.pin(event.target.id(), event.target.position());
      updateLabels();
    });
    cy.on('free', 'node', (event) => {
      layout?.pin(event.target.id(), null); grabbedGid = null; hoveredGid = null;
      container.style.cursor = ''; updateFocus();
    });
    cy.on('zoom pan', () => { if (!fitting && initialFit) userViewport = true; updateLabels(); });
    cy.on('zoom', scheduleViewportStyles);
    let elementIndex = 0;
    const addElements = () => {
      addFrame = 0;
      if (disposed) return;
      const started = performance.now();
      do {
        cy.add(elements.slice(elementIndex, elementIndex + 200));
        elementIndex += 200;
      } while (elementIndex < elements.length && performance.now() - started < 8);
      if (elementIndex < elements.length) { addFrame = requestAnimationFrame(addElements); return; }
      layout = startGraphLayout({
        nodes: cy.nodes().map((node) => ({ id: node.id(), radius: node.data('diameter') / 2, cluster: node.data('cluster') })),
        edges: graph.edges.map(({ source, target }) => ({ source, target })), previous: [...positionsRef.current],
        forces: optionsRef.current.forces, motion: canAnimate(),
      }, (frame) => {
        if (disposed) return;
        if (frame.ids) ids = frame.ids;
        latestFrame = frame;
        if (!renderFrame) renderFrame = requestAnimationFrame(renderPositions);
      });
      updateFocus();
    };
    addFrame = requestAnimationFrame(addElements);
    const onMotionPreference = () => { if (reducedMotion.matches) setMotion(false); resume(); };
    reducedMotion.addEventListener('change', onMotionPreference);
    const resize = new ResizeObserver(() => {
      cancelAnimationFrame(resizeFrame);
      resizeFrame = requestAnimationFrame(() => { cy.resize(); updateLabels(); });
    });
    resize.observe(container);
    return () => {
      disposed = true;
      layout?.dispose();
      if (latestFrame) positionsRef.current = new Map(ids.map((id, i) => [id, { x: latestFrame!.positions[i * 2], y: latestFrame!.positions[i * 2 + 1] }]));
      resize.disconnect();
      reducedMotion.removeEventListener('change', onMotionPreference);
      cancelAnimationFrame(addFrame); cancelAnimationFrame(resizeFrame); cancelAnimationFrame(labelFrame); cancelAnimationFrame(renderFrame);
      window.clearTimeout(styleTimer);
      refreshLabelsRef.current = null; updateForcesRef.current = null; updateFocusRef.current = null;
      cy.destroy();
      // A new local/global graph gets its own renderer; release the old GPU resources now.
      gpuContext?.getExtension('WEBGL_lose_context')?.loseContext();
      if (cyRef.current === cy) cyRef.current = null;
    };
  }, [graph, hasNodes]);

  useEffect(() => { updateFocusRef.current?.(); }, [selectedGid, graph, hasNodes]);
  useEffect(() => { refreshLabelsRef.current?.(); }, [showContext]);
  useEffect(() => { updateForcesRef.current?.(); }, [forces, motion]);

  const zoomBy = (factor: number) => {
    const cy = cyRef.current;
    if (!cy) return;
    cy.zoom({ level: Math.max(cy.minZoom(), Math.min(cy.maxZoom(), cy.zoom() * factor)),
      renderedPosition: { x: cy.width() / 2, y: cy.height() / 2 } });
  };
  const stateMessage = loading ? t("Загрузка графа…") : !graph ? t("Граф пока не загружен")
    : !graph.nodes.length ? (isGlobal ? t("В этой сети пока нет клиентов") : t("Для выбранного клиента связи не найдены")) : null;
  const sliders: { key: keyof ForceSettings; label: string }[] = [
    { key: 'repulsion', get label() { return t("Отталкивание"); } }, { key: 'distance', get label() { return t("Длина связей"); } }, { key: 'center', get label() { return t("Притяжение к центру"); } },
  ];

  return (
    <section className="aml-graph" aria-label={t("Граф денежных переводов")}>
      <div className="aml-graph__header">
        <p>{t("Стрелка → получатель")}<span className="aml-graph__label-hint">{" "}{t("· Потяните узел, чтобы исследовать связи")}</span></p>
        {hasNodes && <div className="aml-graph__controls">
          <div className="aml-graph__zoom">
            <button type="button" aria-label={t("Уменьшить граф")} disabled={zoomPercent <= minZoom * 100} onClick={() => zoomBy(1 / 1.35)}>−</button>
            <output aria-label={t("Масштаб графа")}>{zoomPercent}%</output>
            <button type="button" aria-label={t("Увеличить граф")} disabled={zoomPercent >= 400} onClick={() => zoomBy(1.35)}>+</button>
          </div>
          <button type="button" className="aml-graph__fit" aria-label={isGlobal ? t("Вписать всю сеть") : t("Вписать окружение клиента")} onClick={() => { if (cyRef.current) fitGraph(cyRef.current, isGlobal); }}>{isGlobal ? t("Вся сеть") : t("Весь срез")}</button>
          <button type="button" className={'aml-graph__settings-button' + (settingsOpen ? ' is-active' : '')}
            aria-label={t("Настройки графа")} aria-expanded={settingsOpen} aria-controls={settingsId}
            onClick={() => setSettingsOpen((value) => !value)}><SettingsIcon /></button>
        </div>}
      </div>
      {stateMessage ? <div className="aml-graph__state" role="status">{stateMessage}</div> : <>
        {selectedMissing && <p className="aml-graph__notice" role="status">{t("Выбранный gid отсутствует в показанном срезе.")}</p>}
        <div className="aml-graph__viewport" aria-busy={layoutBusy}>
          {layoutBusy && <div className="aml-graph__layout-status" role="status">{t("Размещаем")}{" "}{graph!.nodes.length.toLocaleString(getLocale())}{" "}{t("узлов…")}</div>}
          <div ref={containerRef} className="aml-graph__canvas" tabIndex={0}
            aria-label={t("Ориентированный граф переводов. Перетаскивайте узлы и фон. Плюс и минус — масштаб, стрелки — перемещение, 0 — весь срез.")}
            onKeyDown={(event) => {
              const cy = cyRef.current; if (!cy) return;
              const pan: Record<string, Point> = { ArrowLeft: { x: 40, y: 0 }, ArrowRight: { x: -40, y: 0 }, ArrowUp: { x: 0, y: 40 }, ArrowDown: { x: 0, y: -40 } };
              if (event.key === '+' || event.key === '=') zoomBy(1.35);
              else if (event.key === '-') zoomBy(1 / 1.35);
              else if (event.key === '0') fitGraph(cy, isGlobal);
              else if (pan[event.key]) cy.panBy(pan[event.key]);
              else return;
              event.preventDefault();
            }} />
          <svg className="aml-graph__leaders" aria-hidden="true">{labels.filter((label) => label.kind !== 'context').map((label) => <line key={label.gid}
            x1={label.anchorX} y1={label.anchorY}
            x2={Math.max(label.left, Math.min(label.anchorX, label.left + label.width))}
            y2={Math.max(label.top, Math.min(label.anchorY, label.top + label.height))} />)}</svg>
          <div className="aml-graph__labels" aria-hidden="true">{labels.map((label) => <span key={label.gid}
            className={'aml-graph__label aml-graph__label--' + label.kind}
            style={{ left: label.left, top: label.top, width: label.width, height: label.height,
              opacity: label.kind === 'context' ? Math.min(1, Math.max(0, (zoomPercent / 100 - 0.3) / 0.45)) : 1 }}>{label.text}</span>)}</div>
          {settingsOpen && <aside id={settingsId} className="aml-graph__settings" aria-label={t("Настройки отображения графа")}
            onKeyDown={(event) => { if (event.key === 'Escape') setSettingsOpen(false); }}>
            <div className="aml-graph__settings-heading"><strong>{t("Граф")}</strong><button type="button"
              aria-label={t("Сбросить настройки графа")} onClick={() => { setForces({ ...defaultForces }); setShowContext(true); setMotion(!window.matchMedia('(prefers-reduced-motion: reduce)').matches); }}>{t("Сбросить")}</button></div>
            <label className="aml-graph__option"><span>{t("Подписи узлов")}</span><input type="checkbox" checked={showContext} onChange={(event) => setShowContext(event.target.checked)} /></label>
            <label className="aml-graph__option"><span>{t("Плавное движение")}</span><input type="checkbox" checked={motion} onChange={(event) => setMotion(event.target.checked)} /></label>
            <div className="aml-graph__settings-divider" />
            {sliders.map(({ key, label }) => <label className="aml-graph__slider" key={key}>
              <span>{label}<output>{forces[key].toFixed(1)}×</output></span>
              <input type="range" min="0.4" max="2.5" step="0.1" value={forces[key]}
                onChange={(event) => setForces((current) => ({ ...current, [key]: Number(event.target.value) }))} />
            </label>)}
            <p>{t("Размер узла — приоритет проверки. Цвет — аналитическая роль.")}</p>
          </aside>}
          <div className="aml-graph__hint" aria-hidden="true">{t("Колесо — масштаб · Фон — перемещение")}</div>
        </div>
        <div className="aml-graph__legend" aria-label={t("Легенда ролей")}>
          {roles.map(({ role, label }) => <span className="aml-graph__legend-item" key={role}>
            <span className="aml-graph__swatch" style={{ backgroundColor: 'var(--role-' + role + ')' }} aria-hidden="true" />{label}
          </span>)}
        </div>
      </>}
    </section>
  );
}
