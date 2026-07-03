// Statický read-only backend pro GitHub Pages: místo /api čte JSON snapshot
// vygenerovaný příkazem `genealogy export` (web/public/data/*.json).
// Vyhledávání, BFS okolí i rodový strom se počítají v prohlížeči.
import type { Graph, GraphEdge, GraphNode, Person, SearchHit } from './api'

interface PersonIdx {
  id: number
  name: string
  sex: string
  birth_year?: number
  death_year?: number
  confidence: number
  mention_count: number
  surname_norms: string[]
  given_norms: string[]
  places?: string[]
}

const cache = new Map<string, Promise<any>>()

function load<T>(file: string): Promise<T> {
  if (!cache.has(file)) {
    const url = `${import.meta.env.BASE_URL}data/${file}`
    cache.set(
      file,
      fetch(url).then((r) => {
        if (!r.ok) throw new Error(`chybí ${url} — spusť \`genealogy export\``)
        return r.json()
      }),
    )
  }
  return cache.get(file)!
}

// fold: diakritika pryč + lowercase (zrcadlí foldASCII v Go)
function fold(s: string): string {
  return s
    .toLowerCase()
    .normalize('NFD')
    .replace(/[̀-ͯ]/g, '')
}

// surnameNormJS zrcadlí Go surnameNorm (bez genitivního kontextu):
// w→v, adjektivní tvary -ská/-ski/-ského → -sky, ženské -ová/-ové → kmen.
export function surnameNormJS(s: string): string {
  let f = fold(s.trim()).replace(/w/g, 'v')
  outer: for (const grp of ['sk', 'ck']) {
    for (const suf of ['eho', 'emu', 'a', 'e', 'i', 'ym', 'ou']) {
      const full = grp + suf
      if (f.endsWith(full)) {
        const stem = f.slice(0, -full.length)
        if (stem.length >= 4 && !'aeiouy'.includes(stem[stem.length - 1])) {
          f = stem + grp + 'y'
          break outer
        }
      }
    }
  }
  if (f.endsWith('ova') || f.endsWith('ove')) f = f.slice(0, -3)
  return f
}

function placeNormJS(s: string): string {
  let f = fold(s.trim()).replace(/w/g, 'v')
  if (f.length > 3 && 'aeiouy'.includes(f[f.length - 1])) f = f.slice(0, -1)
  return f
}

function toNode(p: PersonIdx, depth: number, focus: boolean): GraphNode {
  return {
    id: p.id,
    name: p.name,
    sex: p.sex,
    birth_year: p.birth_year || undefined,
    death_year: p.death_year || undefined,
    confidence: p.confidence,
    depth,
    has_death: (p.death_year ?? 0) > 0,
    focus,
  }
}

async function bfsGraph(seeds: number[], hops: number, focusIds: Set<number>): Promise<Graph> {
  const [personsArr, graph] = await Promise.all([
    load<PersonIdx[]>('persons.json'),
    load<{ edges: GraphEdge[] }>('graph.json'),
  ])
  const persons = new Map(personsArr.map((p) => [p.id, p]))
  const adj = new Map<number, GraphEdge[]>()
  for (const e of graph.edges) {
    if (!adj.has(e.source)) adj.set(e.source, [])
    if (!adj.has(e.target)) adj.set(e.target, [])
    adj.get(e.source)!.push(e)
    adj.get(e.target)!.push(e)
  }
  const dist = new Map<number, number>(seeds.map((s) => [s, 0]))
  const queue = [...seeds]
  while (queue.length) {
    const cur = queue.shift()!
    if (dist.get(cur)! >= hops) continue
    for (const e of adj.get(cur) ?? []) {
      const next = e.source === cur ? e.target : e.source
      if (!dist.has(next)) {
        dist.set(next, dist.get(cur)! + 1)
        queue.push(next)
      }
    }
  }
  const nodes: GraphNode[] = []
  for (const [pid, d] of dist) {
    const p = persons.get(pid)
    if (p) nodes.push(toNode(p, d, focusIds.size === 0 || focusIds.has(pid)))
  }
  const edges = graph.edges.filter((e) => dist.has(e.source) && dist.has(e.target))
  return { root: seeds[0] ?? -1, nodes, edges }
}

const READ_ONLY = 'Statická verze (GitHub Pages) je jen ke čtení — merge/split a opravy spusť lokálně přes `genealogy serve`.'

export const staticApi = {
  async search(params: Record<string, string>): Promise<SearchHit[]> {
    const persons = await load<PersonIdx[]>('persons.json')
    const toks = fold(params.q ?? '')
      .split(/\s+/)
      .filter(Boolean)
      .map((t) => t.replace(/w/g, 'v'))
    const place = params.place ? placeNormJS(params.place) : ''
    const yFrom = params.year_from ? Number(params.year_from) : null
    const yTo = params.year_to ? Number(params.year_to) : null
    return persons
      .filter((p) => {
        const hay = [...p.surname_norms, ...p.given_norms, fold(p.name)]
        if (!toks.every((t) => hay.some((h) => h.startsWith(t)))) return false
        if (place && !(p.places ?? []).some((pl) => pl.includes(place))) return false
        const y = p.birth_year || p.death_year || null
        if (yFrom !== null && (y ?? 9999) < yFrom) return false
        if (yTo !== null && (y ?? 0) > yTo) return false
        return true
      })
      .sort((a, b) => a.name.localeCompare(b.name, 'cs'))
      .slice(0, 200)
      .map((p) => ({
        id: p.id, name: p.name, sex: p.sex, birth_year: p.birth_year,
        death_year: p.death_year, confidence: p.confidence, mention_count: p.mention_count,
      }))
  },

  async person(id: number): Promise<Person> {
    const details = await load<Record<string, any>>('details.json')
    const d = details[String(id)]
    if (!d) throw new Error(`osoba ${id} neexistuje`)
    return { ...d, candidates: (d.candidates ?? []).map((c: any) => ({ ...c, mention_a: 0, mention_b: 0 })) }
  },

  async neighborhood(id: number, depth = 2): Promise<Graph> {
    return bfsGraph([id], depth, new Set())
  },

  async tree(surname: string, hops = 1): Promise<Graph> {
    const persons = await load<PersonIdx[]>('persons.json')
    const norm = surnameNormJS(surname)
    const focus = persons.filter((p) => p.surname_norms.includes(norm)).map((p) => p.id)
    return bfsGraph(focus, hops, new Set(focus))
  },

  async analytics(kind: string): Promise<any[]> {
    const all = await load<Record<string, any[]>>('analytics.json')
    return all[kind] ?? []
  },

  async stats(): Promise<Record<string, number>> {
    return load('stats.json')
  },

  record: () => Promise.reject(new Error(READ_ONLY)),
  patchCell: () => Promise.reject(new Error(READ_ONLY)),
  merge: () => Promise.reject(new Error(READ_ONLY)),
  split: () => Promise.reject(new Error(READ_ONLY)),
  rematch: () => Promise.reject(new Error(READ_ONLY)),
}
