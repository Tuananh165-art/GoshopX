import { useState } from 'react'
import { RecommendationChatPage } from './RecommendationChatPage'

export function ChatBubble() {
  const [open, setOpen] = useState(false)
  const [minimized, setMinimized] = useState(false)

  if (!open) return <button type="button" className="chat-bubble" aria-label="Mở trợ lý sản phẩm" onClick={() => { setOpen(true); setMinimized(false) }}><span className="chat-bubble-icon">✦</span><span className="chat-bubble-label">Trợ lý sản phẩm</span></button>
  if (minimized) return <button type="button" className="chat-bubble" aria-label="Mở rộng trợ lý sản phẩm" onClick={() => setMinimized(false)}><span className="chat-bubble-icon">✦</span><span className="chat-bubble-label">Trợ lý sản phẩm</span></button>

  return <aside className="chat-widget" aria-label="Trợ lý sản phẩm">
    <header className="chat-widget-header"><div className="chat-agent"><div className="chat-agent-avatar">✦</div><div><strong>Trợ lý sản phẩm</strong><small><i /> Đang trực tuyến · Catalog GoshopX</small></div></div><div className="chat-widget-actions"><button type="button" aria-label="Thu nhỏ" onClick={() => setMinimized(true)}><span className="chat-control-icon">−</span></button><button type="button" aria-label="Tắt" onClick={() => setOpen(false)}><span className="chat-control-icon">×</span></button></div></header>
    <RecommendationChatPage embedded />
  </aside>
}
