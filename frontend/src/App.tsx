import LanguageSelect from './i18n/LanguageSelect';
import { t, getLocale } from './i18n/core.ts';
import { useLocale } from './i18n/react';
import { lazy, Suspense, useEffect, useMemo, useRef, useState, type FormEvent } from 'react';
import NodeCard from './features/workspace/NodeCard';
import PriorityIndicator from './features/workspace/PriorityIndicator';
import GraphStatistics from './features/workspace/GraphStatistics';
import AgentDock from './features/workspace/AgentDock';
import DataImport from './features/workspace/DataImport';
import AnalysisHistory from './features/workspace/AnalysisHistory';
import ExportMenu from './features/workspace/ExportMenu';
import WorkspaceState from './features/workspace/WorkspaceState';
import WorkspaceDock from './features/workspace/WorkspaceDock';
import Icon from './features/workspace/Icon';
import { roleLabels } from './features/workspace/labels';
import { useWorkspace } from './features/workspace/useWorkspace';
import { filterNodes, topNodesCsv, type NodeFilters } from './features/workspace/workspaceModel';
import { isDemo } from './shared/api/workspace';
import type { Role } from './shared/contracts';
import './features/workspace/GraphScope.css';

const GraphView = lazy(() => import('./features/graph/GraphView'));
const initialFilters: NodeFilters = { query: '', cluster: 'all', role: 'all', highOnly: false, order: 'priority' };
const compactWorkspace = () => window.matchMedia('(max-width: 1100px)').matches;

export default function App() {
  useLocale();
  const workspace = useWorkspace();
  const readOnlyDemo = workspace.health?.demo_mode === true;
  const { nodes, selectedGid, selectedNode, detail, graph, loadingTop, loadingGraph, loadingDetail, topError, nodeError } = workspace;
  const [filters, setFilters] = useState(initialFilters);
  const [searchError, setSearchError] = useState('');
  const [announcement, setAnnouncement] = useState('');
  const [listLimit, setListLimit] = useState(40);
  const [compact, setCompact] = useState(compactWorkspace);
  const [leftOpen, setLeftOpen] = useState(() => !compactWorkspace());
  const [rightOpen, setRightOpen] = useState(() => !compactWorkspace());
  const searchRef = useRef<HTMLInputElement>(null);
  const focusSearchPending = useRef(false);
  const workspaceRef = useRef<HTMLElement>(null);
  const clusters = useMemo(() => [...new Set(nodes.map((node) => node.cluster_id))].sort((a, b) => a - b), [nodes]);
  const visible = useMemo(() => filterNodes(nodes, filters), [nodes, filters]);
  const filtered = filters.query !== '' || filters.cluster !== 'all' || filters.role !== 'all' || filters.highOnly;
  const highCount = nodes.filter((node) => node.priority_score >= 0.8).length;

  useEffect(() => { setListLimit(40); }, [filters, workspace.analysisId]);
  useEffect(() => { setFilters(initialFilters); setSearchError(''); }, [workspace.analysisId]);

  useEffect(() => {
    const query = window.matchMedia('(max-width: 1100px)');
    const resize = () => {
      setCompact(query.matches);
      if (query.matches) { setLeftOpen(false); setRightOpen(false); }
    };
    query.addEventListener('change', resize);
    return () => query.removeEventListener('change', resize);
  }, []);
  useEffect(() => {
    if (leftOpen && focusSearchPending.current) {
      searchRef.current?.focus();
      focusSearchPending.current = false;
    }
  }, [leftOpen]);
  useEffect(() => {
    const focusSearch = (event: KeyboardEvent) => {
      const target = event.target as HTMLElement;
      if (event.key === '/' && !event.ctrlKey && !event.metaKey && !event.altKey && !target.isContentEditable && !['INPUT', 'TEXTAREA', 'SELECT'].includes(target.tagName)) {
        event.preventDefault();
        if (leftOpen) searchRef.current?.focus();
        else { focusSearchPending.current = true; setLeftOpen(true); }
        if (compact) setRightOpen(false);
      }
    };
    window.addEventListener('keydown', focusSearch);
    return () => window.removeEventListener('keydown', focusSearch);
  }, [leftOpen, compact]);
  useEffect(() => {
    if (!announcement) return;
    const timer = window.setTimeout(() => setAnnouncement(''), 5000);
    return () => window.clearTimeout(timer);
  }, [announcement]);

  const select = (gid: string) => { workspace.selectNode(gid); setSearchError(''); };
  const search = (event: FormEvent) => {
    event.preventDefault();
    const gid = filters.query.trim();
    if (!gid) { searchRef.current?.focus(); return; }
    if (!nodes.some((node) => node.gid === gid) && !graph?.nodes.some((node) => node.gid === gid)) {
      setSearchError(t("Точный gid не найден в доступном наборе. Ниже показаны совпадения."));
      return;
    }
    select(gid);
    setFilters({ ...initialFilters, query: gid });
  };
  const resetFilters = () => { setFilters(initialFilters); setSearchError(''); };
  const closeOverlay = () => {
    const side = leftOpen ? 'left' : 'right';
    setLeftOpen(false); setRightOpen(false);
    requestAnimationFrame(() => workspaceRef.current?.querySelector<HTMLButtonElement>(`.workspace-dock--${side} .workspace-rail`)?.focus());
  };
  const download = () => {
    if (!visible.length) { setAnnouncement(t("В текущем списке нет клиентов для экспорта. Измените фильтры.")); return; }
    try {
      const url = URL.createObjectURL(new Blob([topNodesCsv(visible)], { type: 'text/csv;charset=utf-8;' }));
      const link = document.createElement('a');
      link.href = url;
      link.download = isDemo ? 'top_nodes_demo.csv' : 'filtered_nodes.csv';
      document.body.append(link); link.click(); link.remove();
      window.setTimeout(() => URL.revokeObjectURL(url), 1000);
      setAnnouncement(t("CSV подготовлен: ") + visible.length + t(" клиентов из текущего списка."));
    } catch {
      setAnnouncement(t("Не удалось подготовить CSV. Повторите попытку."));
    }
  };

  return <main className="app-shell">
    <a href="#workspace" className="skip-link">{t("Перейти к анализу")}</a>
    <header className="app-header">
      <a className="brand" href="#workspace" aria-label={t("AMLens — рабочее пространство")}>
        <span className="brand__symbol"><Icon name="network" size={22} /></span>
        <span>AMLens<span className="brand__suffix"> / intelligence</span></span>
      </a>
      <div className="app-header__meta"><LanguageSelect /><span className="workspace-label"><Icon name="shield" size={15} /> {t("AML workspace")}</span><span className="mode-status"><i />{isDemo || readOnlyDemo ? t("Демонстрационный режим") : t("Режим сервиса")}</span></div>
    </header>

    <div className="workspace-title">
      <div><p className="eyebrow">{t("АНАЛИЗ ТРАНЗАКЦИОННОЙ СЕТИ")}</p><h1>{t("За переводами — связи.")}</h1><p className="workspace-title__description">{t("Находите ключевых участников и исследуйте движение средств.")}</p></div>
      <ExportMenu analysisId={workspace.analysisId} onFiltered={download} onStale={workspace.refreshTop} announce={setAnnouncement} disabled={loadingTop || (isDemo && !visible.length)} />
    </div>

    {readOnlyDemo ? <div className="synthetic-notice">{t("Синтетическое демо · Все участники и переводы вымышлены. Доступны граф, карточки и CSV. Импорт и ИИ отключены.")}</div> : <DataImport startImport={workspace.startImport} />}
    {!isDemo && !readOnlyDemo && <AnalysisHistory analysisId={workspace.analysisId} onOpen={workspace.refreshTop} />}
    {!isDemo && <div className={'analysis-status' + (topError ? ' analysis-status--error' : '')} role="status">
      <span><Icon name={topError ? 'info' : 'shield'} size={15} />{topError || (loadingTop ? t("Подключение к сервису…") : workspace.analysisId ? t("Анализ готов") : t("Сервис готов. Импортируйте три файла для расчёта."))}</span>
      {workspace.analysisId && <span className="analysis-status__version" title={workspace.analysisId}>{t("Версия")}{" "}{workspace.analysisId.slice(0, 8)} · {workspace.snapshot?.summary.n_transactions.toLocaleString(getLocale())}{" "}{t("транзакций")}{topError ? t(" · предыдущий результат") : ''}</span>}
      <button className="text-button" disabled={loadingTop} type="button" onClick={workspace.refreshTop}><Icon name="refresh" size={13} />{t("Обновить")}</button>
    </div>}
    <GraphStatistics graph={workspace.fullGraph} loading={loadingGraph} full={!isDemo} />

    <section ref={workspaceRef} className="network-workspace panel" id="workspace" tabIndex={-1} aria-labelledby="network-title"
      onKeyDown={(event) => { if (event.key === 'Escape' && compact && (leftOpen || rightOpen)) { event.preventDefault(); closeOverlay(); } }}>
      <div className="network-heading"><div className="network-heading__title"><span className="network-symbol"><Icon name="network" size={21} /></span><div><p className="section-kicker">{t("РАБОЧЕЕ ПРОСТРАНСТВО")}</p><h2 id="network-title">{t("Сеть переводов")}</h2></div></div><span className="graph-count">{graph ? graph.nodes.length + t(" узлов · ") + graph.edges.length + t(" связей") : t("Нет среза")}</span></div>
      <div className="workspace-grid" data-left-open={leftOpen} data-right-open={rightOpen}>
      <WorkspaceDock side="left" title={t("Приоритеты")} open={leftOpen} busy={loadingTop} count={loadingTop ? '…' : nodes.length}
        inactive={compact && rightOpen} onToggle={() => { setLeftOpen(!leftOpen); if (compact) setRightOpen(false); }}>
        <form className="search-form" onSubmit={search}>
          <label htmlFor="gid-search" className="sr-only">{t("Поиск по gid")}</label>
          <div className="search-field"><Icon name="search" size={17} /><input ref={searchRef} id="gid-search" value={filters.query} onChange={(event) => { setFilters({ ...filters, query: event.target.value }); setSearchError(''); }} placeholder={t("Найти клиента по gid")} autoComplete="off" spellCheck={false} aria-invalid={!!searchError} aria-describedby={searchError ? 'search-error' : undefined} />
            {filters.query ? <button className="icon-button" type="button" aria-label={t("Очистить поиск")} onClick={() => { setFilters({ ...filters, query: '' }); setSearchError(''); searchRef.current?.focus(); }}><Icon name="close" size={14} /></button> : <kbd aria-hidden="true">/</kbd>}
          </div>
          <button type="submit" className="sr-only" tabIndex={-1}>{t("Найти точный gid")}</button>
        </form>
        {searchError && <p className="inline-error" id="search-error" role="alert">{t(searchError)}</p>}
        <div className="list-filters">
          <label>{t("Кластер в списке")}<select aria-label={t("Кластер в списке")} value={filters.cluster} onChange={(event) => setFilters({ ...filters, cluster: event.target.value })}><option value="all">{t("Все кластеры")}</option>{clusters.map((id) => <option key={id} value={id}>{t("Кластер")}{" "}{id}</option>)}</select></label>
          <label>{t("Роль")}<select aria-label={t("Роль в списке")} value={filters.role} onChange={(event) => setFilters({ ...filters, role: event.target.value as Role | 'all' })}><option value="all">{t("Все роли")}</option>{Object.entries(roleLabels).map(([role, label]) => <option value={role} key={role}>{label}</option>)}</select></label>
        </div>
        <div className="list-toolbar"><label className="priority-filter"><input type="checkbox" checked={filters.highOnly} onChange={(event) => setFilters({ ...filters, highOnly: event.target.checked })} /><span>{t("Приоритет ≥ 0,80")}</span><span className="secondary">{highCount}</span></label>{filtered && <button type="button" className="text-button" onClick={resetFilters}>{t("Сбросить")}</button>}</div>
        <div className="list-caption"><span>{visible.length}{" "}{t("из")}{" "}{nodes.length}</span><label><span className="sr-only">{t("Порядок списка")}</span><select aria-label={t("Порядок списка")} value={filters.order} onChange={(event) => setFilters({ ...filters, order: event.target.value as NodeFilters['order'] })}><option value="priority">{t("По приоритету ↓")}</option><option value="rank">{t("По рангу")}</option></select></label></div>
        <div className="top-list" aria-label={t("Ранжированный список клиентов")}>
          {loadingTop && !nodes.length ? <WorkspaceState title={t("Загрузка участников…")} />
            : topError && !nodes.length ? <WorkspaceState title={t("Список недоступен")} error>{t(topError)}<button type="button" onClick={workspace.refreshTop}><Icon name="refresh" />{t("Повторить")}</button></WorkspaceState>
            : !nodes.length ? <WorkspaceState title={t("Нет загруженного анализа")}>{t("Выберите три файла в разделе «Данные исследования» и запустите расчёт.")}</WorkspaceState>
            : !visible.length ? <WorkspaceState title={t("Нет совпадений")}><p>{t("Измените gid или условия отбора.")}</p><button type="button" onClick={resetFilters}>{t("Сбросить фильтры")}</button></WorkspaceState>
            : visible.slice(0, listLimit).map((node) => <button type="button" key={node.gid} className={'top-item' + (node.gid === selectedGid ? ' top-item--selected' : '')} onClick={() => select(node.gid)} aria-pressed={node.gid === selectedGid}>
              <span className="top-item__line"><span className="top-item__rank">{String(node.rank).padStart(2, '0')}</span><span className="data-text">{node.gid}</span><PriorityIndicator score={node.priority_score} /></span>
              <span className="top-item__role">{roleLabels[node.role]}<span>{t("Кластер")}{" "}{node.cluster_id}</span></span>
              <span className="top-item__why">{t(node.why)}</span>
            </button>)}
          {visible.length > listLimit && <button className="list-more" type="button" onClick={() => setListLimit((value) => value + 40)}>{t("Показать ещё")}{" "}{Math.min(40, visible.length - listLimit)}</button>}
        </div>
        <div className="panel-footer"><Icon name="info" size={14} />{t("Фильтры применяются к списку участников")}</div>
      </WorkspaceDock>

      <section className="graph-panel" aria-label={t("Граф сети переводов")} aria-busy={loadingGraph} inert={compact && (leftOpen || rightOpen)}>
        <div className="graph-scope-bar">
          <div className="graph-scope" role="group" aria-label={t("Область графа")}>
            <button type="button" aria-pressed={workspace.graphMode === 'global'} onClick={workspace.showNetwork}>{t("Вся сеть")}</button>
            <button type="button" aria-pressed={workspace.graphMode === 'local'} disabled={!workspace.fullGraph?.nodes.length}
              onClick={workspace.recenter}>{t("Окружение клиента")}</button>
          </div>
          <span>{workspace.graphMode === 'global' ? isDemo ? t("Все связи демонабора") : t("Все клиенты и связи анализа") : t("Непосредственные связи клиента")}</span>
        </div>
        <div className="graph-context"><span><span className="live-dot" />{workspace.graphMode === 'global' ? t("Общий вид сети") : graph ? t("Окружение ") + graph.center_gid : t("Окружение клиента")}</span>
          <span className="graph-selection"><span className="data-text">{selectedGid ? t("Выбран ") + selectedGid : t("Выберите узел, чтобы открыть карточку")}</span>
            {workspace.graphMode === 'global' && selectedGid && <button type="button" className="text-button" onClick={workspace.showNetwork}>{t("Снять выделение")}</button>}</span></div>
        {workspace.graphMode === 'local' && graph && selectedGid && selectedGid !== graph.center_gid && <div className="graph-slice-notice"><button className="text-button" type="button" onClick={workspace.recenter}>{t("Открыть окружение выбранного клиента")}<Icon name="arrow" size={13} /></button></div>}
        {workspace.graphMode === 'local' && workspace.totalNeighbors > workspace.shownNeighbors && <div className="graph-slice-notice"><span>{t("Показано")}{" "}{workspace.shownNeighbors}{" "}{t("из")}{" "}{workspace.totalNeighbors}{" "}{t("соседей по обороту переводов.")}</span><button className="text-button" type="button" onClick={workspace.showMoreNeighbors}>{t("Ещё")}{" "}{Math.min(120, workspace.totalNeighbors - workspace.shownNeighbors)}</button></div>}
        {nodeError && !graph ? <WorkspaceState title={t("Не удалось получить срез")} error>{t(nodeError)}<button type="button" onClick={workspace.refreshNode}>{t("Повторить запрос")}</button></WorkspaceState>
          : loadingGraph ? <WorkspaceState title={t("Загрузка графа…")} />
          : !isDemo && topError && !graph ? <WorkspaceState title={t("Не удалось подключиться к анализу")} error>{t(topError)}<button type="button" onClick={workspace.refreshTop}>{t("Повторить подключение")}</button></WorkspaceState>
          : !isDemo && !workspace.analysisId ? <WorkspaceState title={t("Загрузите данные исследования")}><Icon name="network" size={36} /><p>{t("Откройте «Импорт файлов» выше и добавьте nodes.parquet, edges.parquet и transactions.parquet. После расчёта здесь появится сеть переводов.")}</p></WorkspaceState>
          : !graph ? <WorkspaceState title={t("Срез для этого клиента пока недоступен")}><Icon name="network" size={36} /><p>{isDemo ? t("В демонаборе нет связей для этого клиента. Известные признаки доступны в панели «Карточка клиента».") : t("Сервис пока не вернул связи выбранного клиента.")}</p><button className="primary-button" type="button" onClick={workspace.showNetwork}>{t("Открыть всю сеть")}<Icon name="arrow" size={16} /></button></WorkspaceState>
          : !graph.nodes.length ? <WorkspaceState title={t("Связи не найдены")}>{t("В доступном срезе нет узлов для отображения.")}</WorkspaceState>
          : <Suspense fallback={<WorkspaceState title={t("Подготовка графа…")} />}><GraphView graph={graph} selectedGid={selectedGid || null} loading={false} onSelectGid={select} /></Suspense>}
        <div className="graph-panel__note"><Icon name="info" size={14} /><span>{t("Роль участника — гипотеза для проверки.")}</span><span className="graph-interaction-hint">{t("Колесо — масштаб · перетаскивание — обзор")}</span></div>
      </section>

      <WorkspaceDock side="right" title={t("Карточка клиента")} open={rightOpen} busy={loadingDetail}
        inactive={compact && leftOpen} onToggle={() => { setRightOpen(!rightOpen); if (compact) setLeftOpen(false); }}>
        {nodeError ? <WorkspaceState title={t("Карточка недоступна")} error>{t(nodeError)}<button type="button" onClick={workspace.refreshNode}>{t("Повторить")}</button></WorkspaceState>
          : loadingDetail ? <WorkspaceState title={t("Загрузка карточки…")} />
          : selectedNode ? <NodeCard key={selectedGid} node={selectedNode} detail={detail} graph={graph} card={workspace.card} />
          : <WorkspaceState title={t("Выберите клиента")}>{t("Нажмите на участника в списке или на узел графа.")}</WorkspaceState>}
      </WorkspaceDock>
      {compact && (leftOpen || rightOpen) && <button className="workspace-backdrop" type="button" tabIndex={-1} aria-label={t("Закрыть боковую панель")} onClick={closeOverlay} />}
      </div>
    </section>
    <AgentDock analysisId={workspace.analysisId} selectedGid={selectedGid || null} configured={workspace.health?.ai_configured ?? null}
      healthLoading={workspace.healthLoading} healthError={workspace.healthError} onRefreshHealth={workspace.refreshHealth}
      isDemo={isDemo || readOnlyDemo} onSelectGid={select} onStale={workspace.refreshTop} />
    <div className="toast" role="status" aria-live="polite" hidden={!announcement}>{t(announcement)}</div>
  </main>;
}
