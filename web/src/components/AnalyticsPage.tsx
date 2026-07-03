import { useEffect, useState } from 'react'
import {
  BarChart, Bar, LineChart, Line, XAxis, YAxis, CartesianGrid, Tooltip, Legend,
  ResponsiveContainer,
} from 'recharts'
import { api } from '../api'

// barvy vrstev/sérií (validovaná kategorická paleta, přiřazení dle entity)
const C = { birth: '#2a78d6', marriage: '#1baf7a', death: '#e34948', male: '#2a78d6', female: '#e87ba4', single: '#2a78d6' }
const INK2 = '#52514e'
const GRID = '#e1e0d9'

const MONTHS = ['', 'led', 'úno', 'bře', 'dub', 'kvě', 'čvn', 'čvc', 'srp', 'zář', 'říj', 'lis', 'pro']

// AnalyticsPage: přehledové grafy nad extrahovanou vrstvou. Každý graf se
// počítá na serveru v SQL — po opravách/re-matchi stačí reload.
export default function AnalyticsPage() {
  const [data, setData] = useState<Record<string, any[]>>({})

  useEffect(() => {
    const kinds = ['timeline', 'surnames', 'lifespan', 'marriage-age', 'seasonality', 'migration', 'family-size']
    kinds.forEach((k) => api.analytics(k).then((rows) => setData((d) => ({ ...d, [k]: rows }))))
  }, [])

  const seasonality = (data['seasonality'] ?? []).map((r) => ({ ...r, name: MONTHS[r.name] ?? r.name }))

  return (
    <div className="analytics">
      <div className="grid">
        <Card title="Události v čase" rows={data['timeline']}>
          <ResponsiveContainer width="100%" height={220}>
            <LineChart data={data['timeline']}>
              <CartesianGrid stroke={GRID} vertical={false} />
              <XAxis dataKey="name" tick={{ fontSize: 11, fill: INK2 }} />
              <YAxis tick={{ fontSize: 11, fill: INK2 }} width={32} />
              <Tooltip />
              <Legend />
              <Line dataKey="birth" name="narození" stroke={C.birth} strokeWidth={2} dot={false} />
              <Line dataKey="marriage" name="svatby" stroke={C.marriage} strokeWidth={2} dot={false} />
              <Line dataKey="death" name="úmrtí" stroke={C.death} strokeWidth={2} dot={false} />
            </LineChart>
          </ResponsiveContainer>
        </Card>

        <Card title="Nejčastější příjmení (podle osob)" rows={data['surnames']}>
          <ResponsiveContainer width="100%" height={220}>
            <BarChart data={(data['surnames'] ?? []).slice(0, 15)}>
              <CartesianGrid stroke={GRID} vertical={false} />
              <XAxis dataKey="name" tick={{ fontSize: 10, fill: INK2 }} angle={-40} textAnchor="end" height={55} />
              <YAxis tick={{ fontSize: 11, fill: INK2 }} width={32} />
              <Tooltip />
              <Bar dataKey="value" name="osob" fill={C.single} radius={[4, 4, 0, 0]} />
            </BarChart>
          </ResponsiveContainer>
        </Card>

        <Card title="Délka života (dekády)" rows={data['lifespan']}>
          <ResponsiveContainer width="100%" height={220}>
            <BarChart data={data['lifespan']}>
              <CartesianGrid stroke={GRID} vertical={false} />
              <XAxis dataKey="name" tick={{ fontSize: 11, fill: INK2 }} tickFormatter={(v) => `${v}+`} />
              <YAxis tick={{ fontSize: 11, fill: INK2 }} width={32} />
              <Tooltip labelFormatter={(v) => `${v}–${Number(v) + 9} let`} />
              <Bar dataKey="value" name="osob" fill={C.single} radius={[4, 4, 0, 0]} />
            </BarChart>
          </ResponsiveContainer>
        </Card>

        <Card title="Věk při sňatku (průměr po dekádách)" rows={data['marriage-age']}>
          <ResponsiveContainer width="100%" height={220}>
            <LineChart data={data['marriage-age']}>
              <CartesianGrid stroke={GRID} vertical={false} />
              <XAxis dataKey="name" tick={{ fontSize: 11, fill: INK2 }} />
              <YAxis tick={{ fontSize: 11, fill: INK2 }} width={32} domain={['auto', 'auto']} />
              <Tooltip />
              <Legend />
              <Line dataKey="zenich" name="ženich" stroke={C.male} strokeWidth={2} />
              <Line dataKey="nevesta" name="nevěsta" stroke={C.female} strokeWidth={2} />
            </LineChart>
          </ResponsiveContainer>
        </Card>

        <Card title="Sezónnost událostí (měsíce)" rows={seasonality}>
          <ResponsiveContainer width="100%" height={220}>
            <BarChart data={seasonality}>
              <CartesianGrid stroke={GRID} vertical={false} />
              <XAxis dataKey="name" tick={{ fontSize: 11, fill: INK2 }} />
              <YAxis tick={{ fontSize: 11, fill: INK2 }} width={32} />
              <Tooltip />
              <Legend />
              <Bar dataKey="birth" name="narození" fill={C.birth} radius={[4, 4, 0, 0]} />
              <Bar dataKey="marriage" name="svatby" fill={C.marriage} radius={[4, 4, 0, 0]} />
              <Bar dataKey="death" name="úmrtí" fill={C.death} radius={[4, 4, 0, 0]} />
            </BarChart>
          </ResponsiveContainer>
        </Card>

        <Card title="Počet dětí na rodinu" rows={data['family-size']}>
          <ResponsiveContainer width="100%" height={220}>
            <BarChart data={data['family-size']}>
              <CartesianGrid stroke={GRID} vertical={false} />
              <XAxis dataKey="name" tick={{ fontSize: 11, fill: INK2 }} />
              <YAxis tick={{ fontSize: 11, fill: INK2 }} width={32} />
              <Tooltip labelFormatter={(v) => `${v} dětí`} />
              <Bar dataKey="value" name="rodin" fill={C.single} radius={[4, 4, 0, 0]} />
            </BarChart>
          </ResponsiveContainer>
        </Card>

        <Card title="Pohyb mezi obcemi (rodiště → místo oddavek)" rows={data['migration']}>
          <table className="mig">
            <tbody>
              {(data['migration'] ?? []).slice(0, 15).map((r, i) => (
                <tr key={i}>
                  <td>{r.name} → {r.target}</td>
                  <td>{r.value}×</td>
                </tr>
              ))}
            </tbody>
          </table>
        </Card>
      </div>
    </div>
  )
}

function Card({ title, rows, children }: { title: string; rows?: any[]; children: React.ReactNode }) {
  return (
    <div className="card">
      <h3>{title}</h3>
      {rows && rows.length === 0 ? <div className="empty">zatím žádná data</div> : children}
    </div>
  )
}
