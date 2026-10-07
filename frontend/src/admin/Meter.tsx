/** Pegelanzeige: linearer Peak (0…1+) → dB-Skala von -60 bis 0 dBFS. */
export function Meter({ value, label }: { value: number; label?: string }) {
  const db = value > 0.000001 ? 20 * Math.log10(value) : -120
  const pct = Math.max(0, Math.min(100, ((db + 60) / 60) * 100))
  return (
    <div title={label ? `${label}: ${db > -100 ? db.toFixed(0) : '-∞'} dB` : undefined}>
      {label && <div className="note">{label}</div>}
      <div className="meter">
        <i style={{ width: `${pct}%` }} />
      </div>
    </div>
  )
}
