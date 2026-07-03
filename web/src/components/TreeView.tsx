import { useEffect, useRef } from 'react'
import cytoscape from 'cytoscape'
import dagre from 'cytoscape-dagre'
import type { Graph } from '../api'

cytoscape.use(dagre)

const COLORS = {
  male: '#2a78d6',
  female: '#e87ba4',
  unknown: '#898781',
  spouse: '#1baf7a',
  parent: '#898781',
  warn: '#c98500',
  death: '#e34948',
}

export interface TreeViewProps {
  graph: Graph | null
  layers: Set<string>            // aktivní vrstvy: birth|marriage|death
  selected: number | null
  onSelect: (id: number) => void
  onExpand: (id: number) => void // dvojklik = donačíst okolí uzlu
}

// TreeView vykresluje graf osob v Cytoscape s dagre layoutem (generace shora dolů).
// Hrany: rodič→dítě plná šipka, manželé zelená bez šipky. Nízká confidence
// (< 0.85) = čárkovaně oranžově. Vrstvy filtrují hrany podle typu událostí,
// ze kterých hrana pochází.
export default function TreeView({ graph, layers, selected, onSelect, onExpand }: TreeViewProps) {
  const ref = useRef<HTMLDivElement>(null)
  const cyRef = useRef<cytoscape.Core | null>(null)

  useEffect(() => {
    const cy = cytoscape({
      container: ref.current!,
      minZoom: 0.1,
      maxZoom: 4,
      wheelSensitivity: 0.3,
      style: [
        {
          selector: 'node',
          style: {
            label: 'data(label)',
            'text-wrap': 'wrap',
            'text-max-width': '120px',
            'font-size': '10px',
            'text-valign': 'bottom',
            'text-margin-y': 4,
            color: '#0b0b0b',
            width: 26,
            height: 26,
            'background-color': COLORS.unknown,
            'border-width': 2,
            'border-color': '#fcfcfb',
          },
        },
        { selector: 'node[sex="m"]', style: { 'background-color': COLORS.male } },
        { selector: 'node[sex="f"]', style: { 'background-color': COLORS.female } },
        {
          selector: 'node[?lowConf]',
          style: { 'border-style': 'dashed', 'border-color': COLORS.warn, 'border-width': 3 },
        },
        // v rodovém zobrazení: osoby mimo hledané příjmení ztlumit
        { selector: 'node[?dimmed]', style: { opacity: 0.4 } },
        {
          selector: 'node[?hasDeath]',
          style: { 'background-image': deathBadge, 'background-width': '55%', 'background-height': '55%' },
        },
        {
          selector: 'node:selected',
          style: { 'border-color': '#0b0b0b', 'border-width': 3, 'border-style': 'solid' },
        },
        {
          selector: 'edge',
          style: {
            width: 2,
            'line-color': COLORS.parent,
            'curve-style': 'bezier',
            'target-arrow-shape': 'triangle',
            'target-arrow-color': COLORS.parent,
            'arrow-scale': 0.8,
          },
        },
        {
          selector: 'edge[type="spouse"]',
          style: {
            'line-color': COLORS.spouse,
            'target-arrow-shape': 'none',
            width: 3,
          },
        },
        {
          selector: 'edge[?lowConf]',
          style: { 'line-style': 'dashed', 'line-color': COLORS.warn, 'target-arrow-color': COLORS.warn },
        },
        { selector: '.layer-hidden', style: { display: 'none' } },
      ],
    })
    cy.on('tap', 'node', (e) => onSelect(Number(e.target.id())))
    cy.on('dbltap', 'node', (e) => onExpand(Number(e.target.id())))
    cyRef.current = cy
    // při změně velikosti plochy (otevření bočního panelu) přepočítat a znovu vejít
    const ro = new ResizeObserver(() => {
      cy.resize()
      cy.fit(undefined, 60)
    })
    ro.observe(ref.current!)
    return () => {
      ro.disconnect()
      cy.destroy()
      cyRef.current = null
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [])

  // překreslení grafu
  useEffect(() => {
    const cy = cyRef.current
    if (!cy || !graph) return
    cy.elements().remove()
    cy.add(
      graph.nodes.map((n) => ({
        group: 'nodes' as const,
        data: {
          id: String(n.id),
          label: `${n.name}${yearsLabel(n.birth_year, n.death_year)}`,
          sex: n.sex,
          lowConf: n.confidence < 0.85,
          hasDeath: n.has_death,
          dimmed: !n.focus,
        },
      })),
    )
    cy.add(
      graph.edges.map((e) => ({
        group: 'edges' as const,
        data: {
          id: 'e' + e.id,
          source: String(e.source),
          target: String(e.target),
          type: e.type,
          lowConf: e.confidence < 0.85,
          layers: e.layers.join(','),
        },
      })),
    )
    runLayout(cy)
  }, [graph])

  // přepínání vrstev: skryj hrany, jejichž všechny zdrojové události jsou vypnuté
  useEffect(() => {
    const cy = cyRef.current
    if (!cy) return
    cy.edges().forEach((e) => {
      const els: string[] = (e.data('layers') || '').split(',').filter(Boolean)
      const visible = els.length === 0 || els.some((l) => layers.has(l))
      e.toggleClass('layer-hidden', !visible)
    })
    // uzly bez viditelné hrany necháváme — kontext neztrácet
  }, [layers, graph])

  // výběr zvenku (z panelu)
  useEffect(() => {
    const cy = cyRef.current
    if (!cy) return
    cy.$('node:selected').unselect()
    if (selected != null) cy.$id(String(selected)).select()
  }, [selected])

  return <div ref={ref} className="cy-container" />
}

// layout: dagre jen přes rodičovské hrany (manželé nemají tvořit generační skok)
function runLayout(cy: cytoscape.Core) {
  const eles = cy.elements().difference(cy.edges('[type="spouse"]'))
  eles
    .layout({
      name: 'dagre',
      // @ts-expect-error volby cytoscape-dagre nejsou v typech
      rankDir: 'TB',
      nodeSep: 40,
      rankSep: 80,
      animate: false,
    })
    .run()
  cy.fit(undefined, 60)
}

function yearsLabel(b?: number, d?: number) {
  if (b && d) return `\n(${b}–${d})`
  if (b) return `\n(*${b})`
  if (d) return `\n(†${d})`
  return ''
}

// malý křížek pro osoby se záznamem úmrtí (SVG data URI)
const deathBadge =
  'data:image/svg+xml;utf8,' +
  encodeURIComponent(
    `<svg xmlns="http://www.w3.org/2000/svg" width="12" height="12"><path d="M6 1v10M2.5 4.5h7" stroke="white" stroke-width="2.4"/></svg>`,
  )
