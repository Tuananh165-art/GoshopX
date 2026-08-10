import { useEffect, useState } from 'react'
import { Link } from 'react-router-dom'
import { graphql, graphqlUpload } from '../../shared/api/graphql'
import { vnd } from '../../shared/lib/format'
import './chatbot.css'

type ChatProduct = {
  id: string
  name: string
  description: string
  price: number
  currency: string
  thumbnail: string
  images: string[]
  rating: number
  stock: number
  matchedReasons: string[]
}

type ChatResponse = {
  answer: string
  products: ChatProduct[]
  nextSuggestions: string[]
  warnings: string[]
  requestId: string
}

type Message = { role: 'user' | 'assistant'; text: string; imageDataUrl?: string; imageName?: string; products?: ChatProduct[]; warnings?: string[]; suggestions?: string[] }

const HISTORY_KEY = 'goshopx-recommendation-chat-history'

function readFileAsDataUrl(file: File): Promise<string> {
  return new Promise((resolve, reject) => {
    const reader = new FileReader()
    reader.onload = () => resolve(String(reader.result || ''))
    reader.onerror = () => reject(reader.error || new Error('Không thể đọc hình ảnh.'))
    reader.readAsDataURL(file)
  })
}

const QUERY = `mutation RecommendChat($input: RecommendationChatInput!) {
  recommendChat(input: $input) {
    answer products { id name description price currency thumbnail images rating stock matchedReasons }
    nextSuggestions warnings requestId
  }
}`

export function RecommendationChatPage({ embedded = false }: { embedded?: boolean }) {
  const [text, setText] = useState('')
  const [image, setImage] = useState<File | null>(null)
  const [imagePreview, setImagePreview] = useState('')
  const [messages, setMessages] = useState<Message[]>([])
  const [historyReady, setHistoryReady] = useState(false)
  const [pending, setPending] = useState(false)
  const [error, setError] = useState('')

  useEffect(() => {
    try {
      const saved = window.localStorage.getItem(HISTORY_KEY)
      if (saved) setMessages(JSON.parse(saved) as Message[])
    } catch {
      window.localStorage.removeItem(HISTORY_KEY)
    } finally {
      setHistoryReady(true)
    }
  }, [])

  useEffect(() => {
    if (!historyReady) return
    try {
      window.localStorage.setItem(HISTORY_KEY, JSON.stringify(messages))
    } catch {
      // A full localStorage quota must not prevent chat responses.
    }
  }, [historyReady, messages])

  async function handleImageChange(event: React.ChangeEvent<HTMLInputElement>) {
    const selected = event.target.files?.[0] || null
    setImage(selected)
    if (selected) {
      try {
        setImagePreview(await readFileAsDataUrl(selected))
      } catch (reason) {
        setImagePreview('')
        setError(reason instanceof Error ? reason.message : 'Không thể đọc hình ảnh.')
      }
    } else {
      setImagePreview('')
    }
  }

  function clearHistory() {
    setMessages([])
    window.localStorage.removeItem(HISTORY_KEY)
  }

  async function submit(event: React.FormEvent) {
    event.preventDefault()
    if (!text.trim() && !image) return
    const requestText = text.trim()
    setMessages(current => [...current, { role: 'user', text: requestText || `Tìm sản phẩm theo hình ảnh ${image?.name || 'đã tải lên'}.`, imageDataUrl: imagePreview, imageName: image?.name }])
    setText('')
    setError('')
    setPending(true)
    try {
      const variables = { input: { text: requestText, imageUrl: null, image: null, limit: 5 } }
      const result = image
        ? await graphqlUpload<{ recommendChat: ChatResponse }>(QUERY, variables, image, 'variables.input.image')
        : await graphql<{ recommendChat: ChatResponse }>(QUERY, variables)
      const response = result.recommendChat
      setMessages(current => [...current, { role: 'assistant', text: response.answer, products: response.products, warnings: response.warnings, suggestions: response.nextSuggestions }])
    } catch (reason) {
      setError(reason instanceof Error ? reason.message : 'Không thể kết nối trợ lý sản phẩm.')
    } finally {
      setPending(false)
      setImage(null)
      setImagePreview('')
    }
  }

  function handleTextKeyDown(event: React.KeyboardEvent<HTMLTextAreaElement>) {
    if (event.key === 'Enter' && !event.shiftKey) {
      event.preventDefault()
      event.currentTarget.form?.requestSubmit()
    }
  }

  return <main className={embedded ? 'chat-page chat-embedded' : 'chat-page'}>
    {!embedded && <header className="chat-header">
      <p className="eyebrow">GOSHOPX PRODUCT ASSISTANT</p>
      <h1>Tìm đúng sản phẩm, nhanh hơn</h1>
      <p>Hỏi về sản phẩm, ngân sách, thương hiệu hoặc tải ảnh từ máy. Trợ lý chỉ trả lời trong phạm vi catalog GoshopX.</p>
    </header>}
    <section className="chat-shell" aria-label="Trợ lý tìm kiếm sản phẩm">
      <div className="chat-toolbar"><div><strong>Lịch sử trò chuyện</strong><span>{messages.length ? `${messages.length} tin nhắn` : 'Bắt đầu cuộc trò chuyện mới'}</span></div><button type="button" onClick={clearHistory} disabled={!messages.length}>Xoá lịch sử</button></div>
      <div className="chat-messages" aria-live="polite">
        {!messages.length && <div className="chat-empty"><div className="chat-empty-icon">✦</div><strong>Chào bạn, tôi có thể giúp gì?</strong><span>Tôi sẽ tìm và so sánh sản phẩm trong catalog GoshopX dựa trên nhu cầu, ngân sách hoặc hình ảnh bạn gửi.</span><div className="chat-prompts"><button type="button" onClick={() => setText('Gợi ý laptop dưới 20 triệu')}>Laptop dưới 20 triệu</button><button type="button" onClick={() => setText('Tìm điện thoại chụp ảnh đẹp')}>Điện thoại chụp ảnh đẹp</button><button type="button" onClick={() => setText('Tìm sản phẩm đang còn hàng')}>Sản phẩm còn hàng</button></div></div>}
        {messages.map((message, index) => <article className={`chat-message ${message.role}`} key={`${message.role}-${index}`}>
          <div className="chat-message-avatar" aria-hidden="true">{message.role === 'assistant' ? '✦' : 'Bạn'}</div>
          <div className="chat-message-body">
          {message.imageDataUrl && <img className="chat-message-image" src={message.imageDataUrl} alt={message.imageName || 'Hình ảnh đã tải lên'} />}
          <p>{message.text}</p>
          {message.warnings?.map(warning => <small className="chat-warning" key={warning}>Lưu ý: {warning}</small>)}
          {message.products?.length ? <div className="chat-products">{message.products.map(product => <div className="chat-product" key={product.id}>
            {product.thumbnail && <img src={product.thumbnail} alt={product.name} loading="lazy" />}
            <div><h2>{product.name}</h2><strong>{vnd(product.price)}</strong><small>{product.stock > 0 ? `Còn ${product.stock} sản phẩm` : 'Tạm hết hàng'}</small><p>{product.matchedReasons?.[0]}</p><Link className="button" to={`/products/${product.id}`}>Xem sản phẩm</Link></div>
          </div>)}</div> : null}
          {message.suggestions?.length ? <div className="chat-suggestions">{message.suggestions.map(suggestion => <button type="button" key={suggestion} onClick={() => setText(suggestion)}>{suggestion}</button>)}</div> : null}
          </div>
        </article>)}
        {pending && <div className="chat-loading">Đang tìm sản phẩm phù hợp…</div>}
      </div>
      <form className="chat-form" onSubmit={submit}>
        <div className="chat-composer">
          <textarea id="chat-text" value={text} onChange={event => setText(event.target.value)} onKeyDown={handleTextKeyDown} placeholder="Hỏi tôi về sản phẩm, ngân sách, thương hiệu…" maxLength={1000} aria-label="Câu hỏi sản phẩm" />
          <div className="chat-composer-footer"><label className="chat-attach" htmlFor="chat-image" title="Tải ảnh sản phẩm"><span aria-hidden="true">⌕</span><span>Ảnh sản phẩm</span></label><input id="chat-image" accept="image/jpeg,image/png,image/webp" onChange={handleImageChange} type="file" /><small>{text.length}/1000 · Enter để gửi</small><button className="primary" disabled={pending || (!text.trim() && !image)} type="submit" aria-label="Gửi câu hỏi">{pending ? '…' : '➤'}</button></div>
        </div>
        {imagePreview && <div className="chat-selected-file"><img className="chat-upload-preview" src={imagePreview} alt="Ảnh sản phẩm đã chọn" /><span>{image?.name}</span><button type="button" onClick={() => { setImage(null); setImagePreview('') }} aria-label="Bỏ ảnh đã chọn">×</button></div>}
        {error && <p className="error">{error}</p>}
      </form>
    </section>
  </main>
}
