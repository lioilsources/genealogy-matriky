import { useCallback, useEffect, useRef, useState } from 'react'
import { api, Graph, Person, Mention, SearchHit, IS_STATIC } from './api'
import TreeView from './components/TreeView'
import PersonPanel from './components/PersonPanel'
import ScanViewer from './components/ScanViewer'
import AnalyticsPage from './components/AnalyticsPage'

const LAYERS = [
  { key: 'birth', label: 'narození', color: '#2a78d6' },
  { key: 'marriage', label: 'svatby', color: '#1baf7a' },
  { key: 'death', label: 'úmrtí', color: '#e34948' },
]

export default function App() {
  const [tab, setTab] = useState<'tree' | 'analytics'>('tree')
  const [graph, setGraph] = useState<Graph | null>(null)
  const [person, setPerson] = useState<Person | null>(null)
  const [layers, setLayers] = useState(new Set(['birth', 'marriage', 'death']))
  const [depth, setDepth] = useState(2)
  const [scan, setScan] = useState<{ scanId: number; recordId: number; title: string } | null>(null)
  const [toast, setToast] = useState('')

  // vyhledávání
  const [q, setQ] = useState('')
  const [filters, setFilters] = useState({ place: '', year_from: '', year_to: '' })
  const [hits, setHits] = useState<SearchHit[] | null>(null)
  const searchTimer = useRef<number>()

  // rodové zobrazení — výchozí zaměření na Vořechovské (vč. Worechowsky/-ski)
  const [clan, setClan] = useState('Vořechovský')

  const loadClan = useCallback(
    async (surname: string) => {
      if (!surname.trim()) return
      try {
        const g = await api.tree(surname.trim(), 1)
        setPerson(null)
        setGraph(g.nodes.length ? g : null)
        if (!g.nodes.length) showToast(`Rod „${surname}" zatím v datech není.`)
      } catch (e) {
        showToast(String(e))
      }
    },
    [],
  )

  // při startu rovnou ukázat rodový strom
  useEffect(() => {
    loadClan('Vořechovský')
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [])

  useEffect(() => {
    if (!q && !filters.place && !filters.year_from && !filters.year_to) {
      setHits(null)
      return
    }
    window.clearTimeout(searchTimer.current)
    searchTimer.current = window.setTimeout(() => {
      api
        .search({ q, ...filters })
        .then(setHits)
        .catch((e) => setToast(String(e)))
    }, 250)
  }, [q, filters])

  const showToast = (msg: string) => {
    setToast(msg)
    window.setTimeout(() => setToast(''), 3500)
  }

  const loadPerson = useCallback(
    async (id: number, reloadGraph = true) => {
      try {
        const [p, g] = await Promise.all([
          api.person(id),
          reloadGraph ? api.neighborhood(id, depth) : Promise.resolve(null),
        ])
        setPerson(p)
        if (g) setGraph(g)
      } catch (e) {
        showToast(String(e))
      }
    },
    [depth],
  )

  const selectHit = (h: SearchHit) => {
    setHits(null)
    setQ('')
    loadPerson(h.id)
  }

  // dvojklik na uzel: rozšířit graf o okolí uzlu (bez přenačtení celku)
  const expand = async (id: number) => {
    try {
      const extra = await api.neighborhood(id, 1)
      setGraph((g) => {
        if (!g) return extra
        const nodeIds = new Set(g.nodes.map((n) => n.id))
        const edgeIds = new Set(g.edges.map((e) => e.id))
        return {
          ...g,
          nodes: [...g.nodes, ...extra.nodes.filter((n) => !nodeIds.has(n.id))],
          edges: [...g.edges, ...extra.edges.filter((e) => !edgeIds.has(e.id))],
        }
      })
    } catch (e) {
      showToast(String(e))
    }
  }

  const openScan = (m: Mention) => {
    if (IS_STATIC) {
      // bez backendu nejsou skeny — otevřít stránku knihy přímo v ebadatelně
      if (m.scan_url) window.open(m.scan_url, '_blank')
      else showToast('Sken není ve statické verzi dostupný.')
      return
    }
    setScan({
      scanId: m.scan_id,
      recordId: m.record_id,
      title: `${m.book_name} — ${m.scan_file}`,
    })
  }

  // po merge/split/opravě: přenačíst osobu i graf (osoba mohla zaniknout — pak vyčistit)
  const refresh = async () => {
    if (!person) return
    try {
      await loadPerson(person.id)
      showToast('Uloženo a přepočítáno.')
    } catch {
      setPerson(null)
      setGraph(null)
      showToast('Uloženo — osoba byla sloučena/rozdělena, vyhledej ji znovu.')
    }
  }

  return (
    <div className="app">
      <div className="topbar">
        <h1>Matriky — rodokmen</h1>
        <div className="tabs">
          <button className={tab === 'tree' ? 'active' : ''} onClick={() => setTab('tree')}>
            Strom
          </button>
          <button className={tab === 'analytics' ? 'active' : ''} onClick={() => setTab('analytics')}>
            Analytika
          </button>
        </div>
        {tab === 'tree' && (
          <>
            <div className="search-box">
              <input
                placeholder="Hledat osobu (jméno)…"
                value={q}
                onChange={(e) => setQ(e.target.value)}
              />
              {hits && (
                <div className="search-results">
                  {hits.length === 0 && <button disabled>nic nenalezeno</button>}
                  {hits.map((h) => (
                    <button key={h.id} onClick={() => selectHit(h)}>
                      {h.name}{' '}
                      <span className="meta">
                        {h.birth_year ? `*${h.birth_year}` : ''}
                        {h.death_year ? ` †${h.death_year}` : ''} · {h.mention_count} zmínek ·{' '}
                        {Math.round(h.confidence * 100)} %
                      </span>
                    </button>
                  ))}
                </div>
              )}
            </div>
            <div className="filters">
              <input
                placeholder="rod (příjmení)"
                value={clan}
                onChange={(e) => setClan(e.target.value)}
                onKeyDown={(e) => e.key === 'Enter' && loadClan(clan)}
              />
              <button className="action" onClick={() => loadClan(clan)}>
                Zobrazit rod
              </button>
              <input
                placeholder="obec"
                value={filters.place}
                onChange={(e) => setFilters({ ...filters, place: e.target.value })}
              />
              <input
                className="year"
                placeholder="rok od"
                value={filters.year_from}
                onChange={(e) => setFilters({ ...filters, year_from: e.target.value })}
              />
              <input
                className="year"
                placeholder="rok do"
                value={filters.year_to}
                onChange={(e) => setFilters({ ...filters, year_to: e.target.value })}
              />
              <select
                value={depth}
                onChange={(e) => setDepth(Number(e.target.value))}
                style={{ padding: '5px', borderRadius: 6, border: '1px solid #c3c2b7', fontSize: 12 }}
              >
                {[1, 2, 3, 4].map((d) => (
                  <option key={d} value={d}>
                    okolí {d}
                  </option>
                ))}
              </select>
            </div>
            <div className="spacer" />
            <div className="chips">
              {LAYERS.map((l) => (
                <button
                  key={l.key}
                  className={`chip ${layers.has(l.key) ? 'on' : 'off'}`}
                  style={{ color: layers.has(l.key) ? l.color : undefined }}
                  onClick={() => {
                    const next = new Set(layers)
                    next.has(l.key) ? next.delete(l.key) : next.add(l.key)
                    setLayers(next)
                  }}
                >
                  <span className="dot" style={{ background: l.color }} />
                  {l.label}
                </button>
              ))}
            </div>
          </>
        )}
      </div>

      {tab === 'tree' ? (
        <div className="main">
          <div className="tree-area">
            <TreeView
              graph={graph}
              layers={layers}
              selected={person?.id ?? null}
              onSelect={(id) => loadPerson(id, false)}
              onExpand={expand}
            />
            {!graph && <div className="empty-state">Vyhledej osobu a rozklikni její okolí…</div>}
            {graph && (
              <div className="legend">
                <div><span className="swd" style={{ background: '#2a78d6' }} />muž · <span className="swd" style={{ background: '#e87ba4' }} />žena</div>
                <div><span className="sw" style={{ background: '#898781' }} />rodič → dítě · <span className="sw" style={{ background: '#1baf7a' }} />manželé</div>
                <div><span className="sw" style={{ background: '#c98500', height: 2, borderTop: '2px dashed #c98500', backgroundColor: 'transparent' }} />nejistota &lt; 85 % · ✝ má záznam úmrtí</div>
                <div style={{ color: '#898781' }}>dvojklik na uzel = rozšířit okolí</div>
              </div>
            )}
          </div>
          {person && (
            <div className="side-panel">
              <PersonPanel person={person} onOpenScan={openScan} onNavigate={(id) => loadPerson(id)} onChanged={refresh} />
            </div>
          )}
        </div>
      ) : (
        <AnalyticsPage
          onOpenPerson={(id) => {
            setTab('tree')
            loadPerson(id)
          }}
        />
      )}

      {scan && (
        <ScanViewer
          scanId={scan.scanId}
          recordId={scan.recordId}
          title={scan.title}
          onClose={() => setScan(null)}
          onCellSaved={() => person && loadPerson(person.id)}
        />
      )}
      {toast && <div className="toast">{toast}</div>}
    </div>
  )
}
