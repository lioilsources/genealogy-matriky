// Typy a klient pro genealogy API (Go server na /api).

export interface SearchHit {
  id: number
  name: string
  sex: string
  birth_year?: number
  death_year?: number
  confidence: number
  mention_count: number
}

// Statický režim (GitHub Pages): žádné /api, data z JSON snapshotu, jen ke čtení.
export const IS_STATIC = import.meta.env.VITE_STATIC === 'true'

export interface Mention {
  id: number
  role: string
  raw: string
  given: string
  surname: string
  place?: string
  occupation?: string
  birth_year?: number
  age_text?: string
  record_id: number
  row_idx: number
  cislo?: string
  scan_id: number
  scan_file: string
  folio?: string
  book_id: string
  book_name: string
  event_type?: string
  event_year?: number
  event_date?: string
  confidence: number
  scan_url?: string // statický export: odkaz na sken v ebadatelně
}

export interface Candidate {
  person_id: number
  person_name: string
  mention_a: number
  mention_b: number
  score: number
}

export interface Person {
  id: number
  name: string
  sex: string
  birth_year?: number
  death_year?: number
  confidence: number
  mentions: Mention[]
  candidates: Candidate[]
}

export interface GraphNode {
  id: number
  name: string
  sex: string
  birth_year?: number
  death_year?: number
  confidence: number
  depth: number
  has_death: boolean
  focus: boolean // nositel hledaného rodového jména (ostatní UI ztlumí)
}

export interface GraphEdge {
  id: number
  type: 'parent_child' | 'spouse'
  source: number
  target: number
  confidence: number
  layers: string[]
}

export interface Graph {
  root: number
  nodes: GraphNode[]
  edges: GraphEdge[]
}

export interface RecordDetail {
  id: number
  type: string
  cislo: string
  row_idx: number
  cells: Record<string, string>
  corrections: Record<string, string>
  scan_id: number
  scan_file: string
  folio: string
  book_id: string
  book_name: string
}

async function j<T>(res: Response): Promise<T> {
  if (!res.ok) {
    const body = await res.json().catch(() => ({ error: res.statusText }))
    throw new Error(body.error ?? res.statusText)
  }
  return res.json()
}

const httpApi = {
  search: (params: Record<string, string>) =>
    fetch('/api/search?' + new URLSearchParams(params)).then((r) => j<SearchHit[]>(r)),
  person: (id: number) => fetch(`/api/persons/${id}`).then((r) => j<Person>(r)),
  neighborhood: (id: number, depth = 2) =>
    fetch(`/api/persons/${id}/neighborhood?depth=${depth}`).then((r) => j<Graph>(r)),
  tree: (surname: string, hops = 1) =>
    fetch(`/api/tree?surname=${encodeURIComponent(surname)}&hops=${hops}`).then((r) => j<Graph>(r)),
  record: (id: number) => fetch(`/api/records/${id}`).then((r) => j<RecordDetail>(r)),
  patchCell: (recordId: number, key: string, value: string) =>
    fetch(`/api/records/${recordId}/cells`, {
      method: 'PATCH',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ key, value }),
    }).then((r) => j<{ ok: boolean }>(r)),
  merge: (a: number, b: number) =>
    fetch(`/api/persons/${a}/merge/${b}`, { method: 'POST' }).then((r) => j<{ ok: boolean }>(r)),
  split: (id: number, mentionIds: number[]) =>
    fetch(`/api/persons/${id}/split`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ mention_ids: mentionIds }),
    }).then((r) => j<{ ok: boolean }>(r)),
  rematch: () => fetch('/api/match/run', { method: 'POST' }).then((r) => j<{ ok: boolean }>(r)),
  analytics: (kind: string) => fetch(`/api/analytics/${kind}`).then((r) => j<any[]>(r)),
  stats: () => fetch('/api/stats').then((r) => j<Record<string, number>>(r)),
}

// ve statické verzi se všechna čtení obsluhují z JSON snapshotu; mutace hlásí chybu
import { staticApi } from './staticApi'

export const api: typeof httpApi = IS_STATIC ? (staticApi as unknown as typeof httpApi) : httpApi
