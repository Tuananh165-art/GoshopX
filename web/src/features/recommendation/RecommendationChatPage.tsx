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
  brand: string
  sku: string
  warrantyInformation: string
  shippingInformation: string
  returnPolicy: string
}
type ChatSource = { title: string; url: string; publishedAt: string }
type CartProposal = { id: string; baseVersion: string; expiresAt: string; items: { productId: string; quantity: number }[] }
type CartProposalRequest = { accountId: number; expectedCartVersion: string; idempotencyKey: string; items: { productId: string; quantity: number }[] }
type CheckoutQuote = { id: string; subtotalVnd: string; discountVnd: string; totalPriceVnd: string; couponCode: string; expiresAt: string; cartVersion: string }

type ChatResponse = {
  answer: string
  products: ChatProduct[]
  nextSuggestions: string[]
  warnings: string[]
  requestId: string
  sessionId: string
  sources: ChatSource[]
}

type Message = { role: 'user' | 'assistant'; text: string; imageDataUrl?: string; imageName?: string; products?: ChatProduct[]; sources?: ChatSource[]; warnings?: string[]; suggestions?: string[] }

const HISTORY_KEY = 'goshopx-recommendation-chat-history'
const SESSION_KEY = 'goshopx-recommendation-chat-session'
const PROPOSAL_KEY = 'goshopx-recommendation-chat-cart-proposal'

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
    answer products { id name description price currency thumbnail images rating stock matchedReasons brand sku warrantyInformation shippingInformation returnPolicy }
    nextSuggestions warnings requestId sessionId sources { title url publishedAt }
  }
}`

const DELETE_SESSION = `mutation DeleteChatSession($sessionId: String!) {
  deleteChatSession(sessionId: $sessionId)
}`
const CART_VERSION = `query MyCartVersion { myCart { accountId version } }`
const CREATE_CART_PROPOSAL = `mutation CreateCartProposal($expectedCartVersion: String!, $idempotencyKey: String!, $items: [CartProposalItemInput!]!) {
  createCartProposal(expectedCartVersion: $expectedCartVersion, idempotencyKey: $idempotencyKey, items: $items) { id baseVersion expiresAt items { productId quantity } }
}`
const CONFIRM_CART_PROPOSAL = `mutation ConfirmCartProposal($proposalId: String!) { confirmCartProposal(proposalId: $proposalId) { version items { quantity product { id name } } } }`
const PREVIEW_CART_CHECKOUT = `query PreviewCartCheckout($couponCode: String) { previewCartCheckout(couponCode: $couponCode) { id subtotalVnd discountVnd totalPriceVnd couponCode expiresAt cartVersion } }`

export function RecommendationChatPage({ embedded = false }: { embedded?: boolean }) {
  const [sessionId, setSessionId] = useState(() => {
    const saved = window.localStorage.getItem(SESSION_KEY)
    if (saved) return saved
    const created = crypto.randomUUID()
    window.localStorage.setItem(SESSION_KEY, created)
    return created
  })
  const [text, setText] = useState('')
  const [image, setImage] = useState<File | null>(null)
  const [imagePreview, setImagePreview] = useState('')
  const [messages, setMessages] = useState<Message[]>([])
  const [historyReady, setHistoryReady] = useState(false)
  const [pending, setPending] = useState(false)
  const [error, setError] = useState('')
  const [proposal, setProposal] = useState<CartProposal | null>(null)
  const [couponCode, setCouponCode] = useState('')
  const [checkoutQuote, setCheckoutQuote] = useState<CheckoutQuote | null>(null)
  const [quoteError, setQuoteError] = useState('')

  useEffect(() => {
    try {
      const saved = window.localStorage.getItem(HISTORY_KEY)
      if (saved) {
        const history = JSON.parse(saved) as Message[]
        setMessages(history.map(({ imageDataUrl: _imageDataUrl, ...message }) => message))
      }
    } catch {
      window.localStorage.removeItem(HISTORY_KEY)
    } finally {
      setHistoryReady(true)
    }
  }, [])

  useEffect(() => {
    if (!historyReady) return
    const saved = window.localStorage.getItem(PROPOSAL_KEY)
    if (!saved) return
    let request: CartProposalRequest
    try {
      request = JSON.parse(saved) as CartProposalRequest
      if (!Number.isSafeInteger(request.accountId) || request.accountId <= 0 || !request.idempotencyKey || !request.expectedCartVersion || !Array.isArray(request.items) || !request.items.length) throw new Error('invalid proposal')
    } catch {
      window.localStorage.removeItem(PROPOSAL_KEY)
      return
    }
    let active = true
    setPending(true)
    graphql<{ myCart: { accountId: number } }>(CART_VERSION)
      .then(cart => {
        if (cart.myCart.accountId !== request.accountId) throw new Error('Pending cart proposal belongs to another account.')
        return graphql<{ createCartProposal: CartProposal }>(CREATE_CART_PROPOSAL, request)
      })
      .then(result => { if (active) setProposal(result.createCartProposal) })
      .catch(reason => {
        if (!active) return
        window.localStorage.removeItem(PROPOSAL_KEY)
        setError(reason instanceof Error ? reason.message : 'Could not restore the saved cart proposal.')
      })
      .finally(() => { if (active) setPending(false) })
    return () => { active = false }
  }, [historyReady])

  useEffect(() => {
    if (!historyReady) return
    try {
      window.localStorage.setItem(HISTORY_KEY, JSON.stringify(messages.map(({ imageDataUrl: _imageDataUrl, ...message }) => message)))
    } catch {
      // A full localStorage quota must not prevent chat responses.
    }
  }, [historyReady, messages])

  async function handleImageChange(event: React.ChangeEvent<HTMLInputElement>) {
    const selected = event.target.files?.[0] || null
    if (selected && selected.size > 5 * 1024 * 1024) {
      setImage(null)
      setImagePreview('')
      setError('Ảnh tải lên tối đa 5 MiB.')
      event.target.value = ''
      return
    }
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

  async function clearHistory() {
    if (pending || !messages.length) return
    setPending(true)
    setError('')
    try {
      const result = await graphql<{ deleteChatSession: boolean }>(DELETE_SESSION, { sessionId })
      if (!result.deleteChatSession) throw new Error('Máy chủ chưa xác nhận đã xoá lịch sử.')
      setMessages([])
      window.localStorage.removeItem(HISTORY_KEY)
      const nextSessionId = crypto.randomUUID()
      setSessionId(nextSessionId)
      window.localStorage.setItem(SESSION_KEY, nextSessionId)
    } catch (reason) {
      setError(reason instanceof Error ? reason.message : 'Không thể xoá lịch sử trên máy chủ.')
    } finally {
      setPending(false)
    }
  }

  async function createCartProposal(products: ChatProduct[]) {
    if (pending || proposal) return
    const items = products.filter(product => product.stock > 0).slice(0, 20).map(product => ({ productId: product.id, quantity: 1 }))
    if (!items.length) { setError('Các sản phẩm gợi ý hiện không còn hàng để lập đề xuất.'); return }
    setPending(true)
    setError('')
    try {
      const cart = await graphql<{ myCart: { accountId: number; version: string } }>(CART_VERSION)
      const request: CartProposalRequest = {
        accountId: cart.myCart.accountId,
        expectedCartVersion: cart.myCart.version,
        idempotencyKey: crypto.randomUUID(),
        items,
      }
      window.localStorage.setItem(PROPOSAL_KEY, JSON.stringify(request))
      const result = await graphql<{ createCartProposal: CartProposal }>(CREATE_CART_PROPOSAL, request)
      setProposal(result.createCartProposal)
    } catch (reason) {
      setError(reason instanceof Error ? reason.message : 'Không thể tạo đề xuất giỏ hàng.')
    } finally { setPending(false) }
  }

  async function confirmCartProposal() {
    if (!proposal || pending) return
    setPending(true)
    setError('')
    setQuoteError('')
    try {
      await graphql(CONFIRM_CART_PROPOSAL, { proposalId: proposal.id })
      setProposal(null)
      window.localStorage.removeItem(PROPOSAL_KEY)
      try {
        const result = await graphql<{ previewCartCheckout: CheckoutQuote }>(PREVIEW_CART_CHECKOUT, { couponCode: couponCode.trim() || null })
        setCheckoutQuote(result.previewCartCheckout)
        setMessages(current => [...current, { role: 'assistant', text: 'Đã cập nhật giỏ hàng và tạo báo giá xem trước. Giá, tồn kho và mã giảm giá vẫn sẽ được kiểm tra lại khi tạo đơn.' }])
      } catch (reason) {
        setCheckoutQuote(null)
        setQuoteError(reason instanceof Error ? reason.message : 'Giỏ hàng đã cập nhật, nhưng chưa tạo được báo giá. Hãy mở checkout để thử lại.')
        setMessages(current => [...current, { role: 'assistant', text: 'Đã cập nhật giỏ hàng. Báo giá chưa sẵn sàng; bạn có thể mở trang checkout để kiểm tra lại.' }])
      }
    } catch (reason) {
      setError(reason instanceof Error ? reason.message : 'Đề xuất đã thay đổi hoặc hết hạn. Hãy tạo lại từ giỏ hàng hiện tại.')
    } finally { setPending(false) }
  }

  async function submit(event: React.FormEvent) {
    event.preventDefault()
    if (!text.trim() && !image) return
    const requestText = text.trim()
    setMessages(current => [...current, { role: 'user', text: requestText || `Tìm sản phẩm theo hình ảnh ${image?.name || 'đã tải lên'}.`, imageDataUrl: imagePreview || undefined, imageName: image?.name }])
    setText('')
    setError('')
    setPending(true)
    try {
      const variables = { input: { text: requestText, imageUrl: null, image: null, sessionId, limit: 5 } }
      const result = image
        ? await graphqlUpload<{ recommendChat: ChatResponse }>(QUERY, variables, image, 'variables.input.image')
        : await graphql<{ recommendChat: ChatResponse }>(QUERY, variables)
      const response = result.recommendChat
      setSessionId(response.sessionId || sessionId)
      window.localStorage.setItem(SESSION_KEY, response.sessionId || sessionId)
      setMessages(current => [...current, { role: 'assistant', text: response.answer, products: response.products, sources: response.sources, warnings: response.warnings, suggestions: response.nextSuggestions }])
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
            <div><h2>{product.name}</h2><strong>{vnd(product.price)}</strong><small>{product.brand || 'GoshopX'} · {product.stock > 0 ? `Còn ${product.stock} sản phẩm` : 'Tạm hết hàng'}</small><p>{product.matchedReasons?.[0]}</p><dl><dt>Đánh giá</dt><dd>{product.rating ? `${product.rating.toFixed(1)}/5` : 'Chưa có dữ liệu'}</dd><dt>Bảo hành</dt><dd>{product.warrantyInformation || 'Chưa có dữ liệu'}</dd><dt>Giao hàng</dt><dd>{product.shippingInformation || 'Chưa có dữ liệu'}</dd><dt>Đổi trả</dt><dd>{product.returnPolicy || 'Chưa có dữ liệu'}</dd></dl><Link className="button" to={`/products/${product.id}`}>Xem sản phẩm</Link></div>
          </div>)}</div> : null}
          {message.role === 'assistant' && message.products?.length ? <button className="chat-proposal-trigger" type="button" disabled={pending} onClick={() => createCartProposal(message.products || [])}>Tạo đề xuất giỏ hàng</button> : null}
          {message.products && message.products.length > 1 && <details className="chat-comparison"><summary>So sánh {message.products.length} sản phẩm</summary><div className="chat-comparison-scroll"><table><thead><tr><th>Tiêu chí</th>{message.products.map(product => <th key={product.id}>{product.name}</th>)}</tr></thead><tbody>{([['Giá', (p: ChatProduct) => vnd(p.price)], ['Thương hiệu', (p: ChatProduct) => p.brand || 'Chưa có dữ liệu'], ['Đánh giá', (p: ChatProduct) => p.rating ? `${p.rating.toFixed(1)}/5` : 'Chưa có dữ liệu'], ['Bảo hành', (p: ChatProduct) => p.warrantyInformation || 'Chưa có dữ liệu'], ['Đổi trả', (p: ChatProduct) => p.returnPolicy || 'Chưa có dữ liệu']] as const).map(([label, value]) => <tr key={label}><th>{label}</th>{message.products!.map(product => <td key={product.id}>{value(product)}</td>)}</tr>)}</tbody></table></div></details>}
          {message.sources?.length ? <nav className="chat-sources" aria-label="Nguồn tham khảo"><strong>Nguồn tham khảo bổ sung</strong>{message.sources.map((source, sourceIndex) => source.url ? <a key={`${source.url}-${sourceIndex}`} href={source.url} target="_blank" rel="noreferrer">{source.title || source.url}<span aria-hidden="true"> ↗</span></a> : <span className="chat-source-label" key={`${source.title}-${sourceIndex}`}>{source.title}</span>)}</nav> : null}
          {message.suggestions?.length ? <div className="chat-suggestions">{message.suggestions.map(suggestion => <button type="button" key={suggestion} onClick={() => setText(suggestion)}>{suggestion}</button>)}</div> : null}
          </div>
        </article>)}
        {pending && <div className="chat-loading">Đang tìm sản phẩm phù hợp…</div>}
      </div>
      {proposal && <section data-testid="cart-proposal-preview" className="chat-proposal" aria-label="Xem trước đề xuất giỏ hàng"><div><strong>Xem trước giỏ hàng</strong><span>{proposal.items.length} sản phẩm · hết hạn {new Date(proposal.expiresAt).toLocaleTimeString()}</span></div><ul>{proposal.items.map(item => <li key={item.productId}>{messages.flatMap(message => message.products || []).find(product => product.id === item.productId)?.name || item.productId} × {item.quantity}</li>)}</ul><label className="chat-coupon">Mã giảm giá tùy chọn<input value={couponCode} onChange={event => setCouponCode(event.target.value.slice(0, 64))} maxLength={64} autoComplete="off" placeholder="Ví dụ: LAB10" /></label><p>Tồn kho đã được giữ tạm thời. Sau khi xác nhận, trợ lý sẽ xin báo giá mới từ Order; báo giá không giữ lượt coupon.</p><button type="button" disabled={pending} onClick={confirmCartProposal}>{pending ? 'Đang xử lý…' : 'Xác nhận và xem báo giá'}</button></section>}
      {checkoutQuote && <section className="chat-quote" aria-live="polite"><div><strong>Báo giá xem trước</strong><span>Hết hạn {new Date(checkoutQuote.expiresAt).toLocaleTimeString()}</span></div><p><span>Tạm tính</span><b>{vnd(Number(checkoutQuote.subtotalVnd))}</b></p>{Number(checkoutQuote.discountVnd) > 0 && <p><span>Giảm giá{checkoutQuote.couponCode ? ` · ${checkoutQuote.couponCode}` : ''}</span><b>−{vnd(Number(checkoutQuote.discountVnd))}</b></p>}<p className="chat-quote-total"><span>Dự kiến</span><b>{vnd(Number(checkoutQuote.totalPriceVnd))}</b></p><small>Order sẽ kiểm tra lại giá, tồn kho và điều kiện coupon khi tạo đơn.</small><Link to={`/checkout${checkoutQuote.couponCode ? `?coupon=${encodeURIComponent(checkoutQuote.couponCode)}` : ''}`}>Tiếp tục checkout</Link></section>}
      {quoteError && <p className="chat-quote-error" role="alert">{quoteError} <Link to={`/checkout${couponCode.trim() ? `?coupon=${encodeURIComponent(couponCode.trim())}` : ''}`}>Mở checkout</Link></p>}
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
