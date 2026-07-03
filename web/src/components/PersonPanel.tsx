import { useState } from 'react'
import { Person, Mention, api, IS_STATIC } from '../api'

const ROLE_LABELS: Record<string, string> = {
  dite: 'dítě', otec: 'otec', matka: 'matka', kmotr: 'kmotr', babka: 'porodní bába',
  zenich: 'ženich', nevesta: 'nevěsta', svedek: 'svědek', zemrely: 'zemřelý',
  zenich_otec: 'otec ženicha', zenich_matka: 'matka ženicha',
  nevesta_otec: 'otec nevěsty', nevesta_matka: 'matka nevěsty',
  otec_otec: 'děd (po otci)', otec_matka: 'bába (po otci)',
  matka_otec: 'děd (po matce)', matka_matka: 'bába (po matce)',
  zemrely_otec: 'otec zemřelého', zemrely_matka: 'matka zemřelého',
}

const EVENT_LABELS: Record<string, string> = { birth: 'křest', marriage: 'oddavky', death: 'úmrtí' }
const EVENT_COLORS: Record<string, string> = { birth: '#2a78d6', marriage: '#1baf7a', death: '#e34948' }

export interface PersonPanelProps {
  person: Person
  onOpenScan: (m: Mention) => void
  onNavigate: (personId: number) => void
  onChanged: () => void // po merge/split — refresh grafu i osoby
}

// PersonPanel: detail osoby — všechny zmínky s odkazem na zdrojový sken,
// míra jistoty, návrhy možných shod (accept → merge) a split vybraných zmínek.
export default function PersonPanel({ person, onOpenScan, onNavigate, onChanged }: PersonPanelProps) {
  const [splitMode, setSplitMode] = useState(false)
  const [picked, setPicked] = useState<Set<number>>(new Set())
  const [busy, setBusy] = useState(false)

  const lowConf = person.confidence < 0.85

  const doMerge = async (otherId: number) => {
    setBusy(true)
    try {
      await api.merge(person.id, otherId)
      onChanged()
    } finally {
      setBusy(false)
    }
  }

  const doSplit = async () => {
    setBusy(true)
    try {
      await api.split(person.id, [...picked])
      setSplitMode(false)
      setPicked(new Set())
      onChanged()
    } finally {
      setBusy(false)
    }
  }

  return (
    <div className="person-panel">
      <h2>{person.name}</h2>
      <div className="years">
        {person.sex === 'm' ? '♂' : person.sex === 'f' ? '♀' : ''}{' '}
        {person.birth_year ? `*${person.birth_year}` : ''}
        {person.death_year ? ` †${person.death_year}` : ''}
      </div>
      <div className="conf-bar">
        <div
          style={{
            width: `${Math.round(person.confidence * 100)}%`,
            background: lowConf ? '#fab219' : '#1baf7a',
          }}
        />
      </div>
      <div className={`conf-note${lowConf ? ' warn' : ''}`}>
        {lowConf
          ? `⚠ jistota spojení ${Math.round(person.confidence * 100)} % — ověř proti skenům níže`
          : `jistota spojení ${Math.round(person.confidence * 100)} %`}
      </div>

      <div className="panel-section">
        <h3>
          Zmínky v matrikách ({person.mentions.length})
          {!IS_STATIC && person.mentions.length > 1 && (
            <>
              {' · '}
              <button className="action" style={{ padding: '2px 8px' }} onClick={() => setSplitMode(!splitMode)}>
                {splitMode ? 'zrušit' : 'není to táž osoba?'}
              </button>
            </>
          )}
        </h3>
        {person.mentions.map((m) => (
          <div className="mention" key={m.id}>
            <span className="role" style={{ color: EVENT_COLORS[m.event_type ?? ''] ?? '#52514e' }}>
              {ROLE_LABELS[m.role] ?? m.role}
              {m.event_type ? ` · ${EVENT_LABELS[m.event_type]} ${m.event_date || m.event_year || ''}` : ''}
            </span>
            <div className="raw">{m.raw || `${m.given} ${m.surname}`.trim()}</div>
            {m.age_text && <div className="raw">věk: {m.age_text}</div>}
            <div className="src">
              {m.book_name} · sken {m.scan_file}
              {m.folio ? ` · folio ${m.folio}` : ''} · řádek {m.row_idx + 1}{' '}
              <button onClick={() => onOpenScan(m)}>otevřít sken ↗</button>
            </div>
            {splitMode && (
              <label>
                <input
                  type="checkbox"
                  checked={picked.has(m.id)}
                  onChange={(e) => {
                    const next = new Set(picked)
                    e.target.checked ? next.add(m.id) : next.delete(m.id)
                    setPicked(next)
                  }}
                />
                oddělit tuto zmínku do jiné osoby
              </label>
            )}
          </div>
        ))}
        {splitMode && (
          <button
            className="action primary"
            disabled={busy || picked.size === 0 || picked.size === person.mentions.length}
            onClick={doSplit}
          >
            Oddělit vybrané ({picked.size})
          </button>
        )}
      </div>

      {person.candidates.length > 0 && (
        <div className="panel-section">
          <h3>Možné shody (posuď proti skenům)</h3>
          {person.candidates.map((c, i) => (
            <div className="candidate" key={i}>
              <span>
                <button
                  className="action"
                  style={{ border: 'none', padding: 0, textDecoration: 'underline' }}
                  onClick={() => onNavigate(c.person_id)}
                >
                  {c.person_name}
                </button>{' '}
                <span style={{ color: '#898781' }}>skóre {Math.round(c.score * 100)} %</span>
              </span>
              {!IS_STATIC && (
                <button className="action" disabled={busy} onClick={() => doMerge(c.person_id)}>
                  Je to táž osoba
                </button>
              )}
            </div>
          ))}
        </div>
      )}
    </div>
  )
}
