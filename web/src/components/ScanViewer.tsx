import { useEffect, useRef, useState } from 'react'
import OpenSeadragon from 'openseadragon'
import { api, RecordDetail } from '../api'

export interface ScanViewerProps {
  scanId: number
  recordId: number | null // záznam, kvůli kterému se sken otevřel (editor buněk)
  title: string
  onClose: () => void
  onCellSaved: () => void
}

// ScanViewer: deep-zoom prohlížeč původního skenu matriky (OpenSeadragon)
// + editor OCR buněk příslušného záznamu. Oprava buňky se uloží jako
// cell_corrections a kniha se na serveru re-extrahuje.
export default function ScanViewer({ scanId, recordId, title, onClose, onCellSaved }: ScanViewerProps) {
  const osdRef = useRef<HTMLDivElement>(null)
  const [record, setRecord] = useState<RecordDetail | null>(null)
  const [edits, setEdits] = useState<Record<string, string>>({})
  const [saving, setSaving] = useState<string | null>(null)
  const [showEditor, setShowEditor] = useState(false)

  useEffect(() => {
    const viewer = OpenSeadragon({
      element: osdRef.current!,
      tileSources: { type: 'image', url: `/api/scans/${scanId}/full` },
      showNavigator: false,
      maxZoomPixelRatio: 3,
      showNavigationControl: false,
      gestureSettingsMouse: { clickToZoom: false },
    })
    return () => viewer.destroy()
  }, [scanId])

  useEffect(() => {
    if (recordId != null) api.record(recordId).then(setRecord)
  }, [recordId])

  const save = async (key: string) => {
    if (!record) return
    setSaving(key)
    try {
      await api.patchCell(record.id, key, edits[key])
      const fresh = await api.record(record.id)
      setRecord(fresh)
      setEdits((e) => {
        const { [key]: _, ...rest } = e
        return rest
      })
      onCellSaved()
    } finally {
      setSaving(null)
    }
  }

  return (
    <div className="viewer-overlay">
      <div className="viewer-head">
        <strong>{title}</strong>
        {record && (
          <span>
            záznam č. {record.cislo || '?'} (řádek {record.row_idx + 1}
            {record.folio ? `, folio ${record.folio}` : ''})
          </span>
        )}
        <div className="spacer" />
        {record && (
          <button onClick={() => setShowEditor((s) => !s)}>
            {showEditor ? 'Skrýt přepis' : 'Zobrazit / opravit přepis'}
          </button>
        )}
        <button onClick={onClose}>Zavřít ✕</button>
      </div>
      <div className="osd" ref={osdRef} />
      {showEditor && record && (
        <div className="record-editor">
          <table>
            <tbody>
              {Object.entries(record.cells)
                .filter(([k]) => k !== 'record_type')
                .map(([key, value]) => {
                  const corrected = record.corrections[key]
                  const current = edits[key] ?? corrected ?? value
                  return (
                    <tr key={key}>
                      <td className="key">
                        {key}
                        {corrected !== undefined && <div className="corrected">opraveno ručně</div>}
                      </td>
                      <td>
                        <input
                          value={current}
                          onChange={(e) => setEdits((s) => ({ ...s, [key]: e.target.value }))}
                        />
                      </td>
                      <td style={{ width: 70 }}>
                        <button
                          className="action"
                          disabled={edits[key] === undefined || saving === key}
                          onClick={() => save(key)}
                        >
                          {saving === key ? '…' : 'Uložit'}
                        </button>
                      </td>
                    </tr>
                  )
                })}
            </tbody>
          </table>
        </div>
      )}
    </div>
  )
}
