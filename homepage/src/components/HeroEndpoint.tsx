import { Check, Copy } from 'lucide-react'
import { useEffect, useState } from 'react'
import { useReducedMotionPreference } from '../hooks/useReducedMotion'
import './HeroEndpoint.css'

const paths = ['/v1/chat/completions', '/v1/messages']
export function HeroEndpoint({ origin }: { origin: string }) {
  const reduced = useReducedMotionPreference()
  const [path, setPath] = useState(paths[0]!)
  const [copyState, setCopyState] = useState('复制')
  useEffect(() => {
    setPath(paths[0]!)
    if (reduced) return
    let current = 0, length = paths[0]!.length, removing = true, hold = 32
    const timer = window.setInterval(() => {
      if (hold-- > 0) return
      if (removing) {
        length--
        if (!length) { removing = false; current = (current + 1) % paths.length }
      } else {
        length++
        if (length === paths[current]!.length) { removing = true; hold = 32 }
      }
      setPath(paths[current]!.slice(0, length))
    }, 65)
    return () => window.clearInterval(timer)
  }, [reduced])
  useEffect(() => {
    if (copyState === '复制') return
    const timer = window.setTimeout(() => setCopyState('复制'), 1800)
    return () => window.clearTimeout(timer)
  }, [copyState])
  async function copy() {
    try { await navigator.clipboard.writeText(origin); setCopyState('已复制') }
    catch { setCopyState('请选中复制') }
  }
  return <div className="hero-endpoint" aria-label="API 接入地址">
    <div className="hero-endpoint-label"><i aria-hidden="true" />Base URL · API 接入端点</div>
    <div className="hero-endpoint-line">
      <code>{origin}</code>
      <button type="button" onClick={copy} aria-label="复制 API 地址">
        {copyState === '已复制' ? <Check aria-hidden="true" /> : <Copy aria-hidden="true" />}
        <span aria-live="polite">{copyState}</span>
      </button>
      <span className="hero-endpoint-path" aria-hidden="true">{path}<b>▏</b></span>
      <span className="sr-only">支持 /v1/chat/completions 和 /v1/messages</span>
    </div>
  </div>
}
