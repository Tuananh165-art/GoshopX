import { useCallback, useEffect, useMemo, useState } from 'react'
import { Link } from 'react-router-dom'
import { graphql, graphqlUpload } from '../../shared/api/graphql'
import { vnd } from '../../shared/lib/format'

const items = [['', 'Tổng quan'], ['catalog', 'Danh mục'], ['orders', 'Đơn hàng'], ['transactions', 'Giao dịch'], ['inventory', 'Kho hàng'], ['accounts', 'Tài khoản'], ['audit', 'Nhật ký']]
const names: Record<string, string> = { catalog: 'Danh mục & kiểm duyệt', orders: 'Đơn hàng', transactions: 'Giao dịch & đối soát', inventory: 'Kho & phiên giữ hàng', accounts: 'Tài khoản', audit: 'Nhật ký bất biến' }

type Account = { id: number; name: string; email: string; role: string; status: string }
type Category = { id: string; name: string; slug: string; description: string; isActive: boolean }
type AdminProduct = { id: string; name: string; description: string; price: number; categoryId: string; brand: string; sku: string; thumbnail: string; images: string[]; tags: string[]; publishStatus: string; moderationStatus: string; moderationReason: string }
type Order = { id: number; accountId: number; totalPrice: number; status: string; paymentStatus: string; createdAt: string }
type Transaction = { orderId: number; userId: number; paymentId: string; totalPrice: number; settledPrice: number; currency: string; status: string }
type Stock = { productId: string; totalQuantity: number; reservedQuantity: number; availableQuantity: number; reorderLevel: number; lowStock: boolean }
type Reservation = { reservationId: string; accountId: number; productId: string; quantity: number; status: string; source: string; expiresAt: string }
type AuditEvent = { eventId: string; actorAccountId: number; targetAccountId: number; action: string; outcome: string; requestId: string; occurredAt: string }
type Dashboard = { gmv: number; revenue: number; orderCount: number; averageOrderValue: number; paymentAttempts: number; successfulPayments: number; paymentSuccessRate: number }

const accountFields = 'id name email role status'
const orderFields = 'id accountId totalPrice status paymentStatus createdAt'
const transactionFields = 'orderId userId paymentId totalPrice settledPrice currency status'
const stockFields = 'productId totalQuantity reservedQuantity availableQuantity reorderLevel lowStock'
const reservationFields = 'reservationId accountId productId quantity status source expiresAt'
const auditFields = 'eventId actorAccountId targetAccountId action outcome requestId occurredAt'

function formatDate(value: string) { return new Date(value).toLocaleString('vi-VN') }
function statusLabel(value: string) { return value.replaceAll('_', ' ') || '—' }

function Toolbar({ children, onRefresh, loading }: { children?: React.ReactNode; onRefresh: () => void; loading: boolean }) {
  return <div className="admin-toolbar"><input aria-label="Tìm kiếm quản trị" placeholder="Tìm kiếm…" />{children}<button onClick={onRefresh} disabled={loading}>{loading ? 'Đang tải…' : 'Làm mới'}</button></div>
}

function ErrorBox({ message, onRetry }: { message: string; onRetry: () => void }) {
  return <div className="admin-error" role="alert"><b>Không thể tải dữ liệu</b><p>{message}</p><button onClick={onRetry}>Thử lại</button></div>
}

function Dashboard() {
  const [data, setData] = useState<Dashboard | null>(null)
  const [error, setError] = useState('')
  const [loading, setLoading] = useState(true)
  const load = useCallback(() => {
    setLoading(true); setError('')
    const to = new Date(); const from = new Date(to.getTime() - 30 * 24 * 60 * 60 * 1000)
    graphql<{ adminDashboard: Dashboard }>(`query AdminDashboard($from: Time!, $to: Time!) { adminDashboard(from: $from, to: $to) { gmv revenue orderCount averageOrderValue paymentAttempts successfulPayments paymentSuccessRate } }`, { from: from.toISOString(), to: to.toISOString() })
      .then(result => setData(result.adminDashboard)).catch(cause => setError(cause instanceof Error ? cause.message : 'GraphQL không phản hồi.')).finally(() => setLoading(false))
  }, [])
  useEffect(load, [load])
  const metrics = data ? [['GMV', vnd(data.gmv), 'Đơn thanh toán thành công'], ['Doanh thu', vnd(data.revenue), 'Sau phí/hoàn tiền cấu hình'], ['AOV', vnd(data.averageOrderValue), `${data.orderCount} đơn thành công`], ['Tỷ lệ thanh toán', `${(data.paymentSuccessRate * 100).toFixed(1)}%`, `${data.successfulPayments}/${data.paymentAttempts} lần thử hợp lệ`]] : [['GMV', '—', 'Đang tải dữ liệu'], ['Doanh thu', '—', 'Đang tải dữ liệu'], ['AOV', '—', 'Đang tải dữ liệu'], ['Tỷ lệ thanh toán', '—', 'Đang tải dữ liệu']]
  return <><section className="metrics">{metrics.map(([title, value, note]) => <article key={title}><span>{title}</span><b>{loading ? '…' : value}</b><small>{note}</small></article>)}</section><section className="admin-panel"><div className="panel-head"><div><h2>Dữ liệu vận hành</h2><p>Đồng bộ từ read model quản trị qua GraphQL, trong khoảng thời gian 30 ngày gần nhất.</p></div><button onClick={load} disabled={loading}>{loading ? 'Đang tải…' : 'Làm mới'}</button></div>{error && <ErrorBox message={error} onRetry={load}/>}</section></>
}

function AccountsPanel() {
  const [rows, setRows] = useState<Account[]>([]); const [state, setState] = useState<'loading' | 'ready' | 'error'>('loading'); const [error, setError] = useState('')
  const load = useCallback(() => { setState('loading'); graphql<{ adminAccounts: Account[] }>(`query AdminAccounts { adminAccounts(pagination: { skip: 0, take: 100 }) { ${accountFields} } }`).then(r => { setRows(r.adminAccounts); setState('ready') }).catch(e => { setError(e instanceof Error ? e.message : 'GraphQL không phản hồi.'); setState('error') }) }, [])
  useEffect(load, [load])
  async function changeStatus(row: Account) { const mutation = row.status === 'SUSPENDED' ? 'reactivateAccount' : 'suspendAccount'; if (!window.confirm(`${mutation === 'suspendAccount' ? 'Tạm ngưng' : 'Kích hoạt'} tài khoản ${row.email}?`)) return; try { await graphql(`mutation { ${mutation}(id: ${row.id}) }`); load() } catch (e) { window.alert(e instanceof Error ? e.message : 'Thao tác thất bại.') } }
  if (state === 'error') return <ErrorBox message={error} onRetry={load}/>
  return <><Toolbar onRefresh={load} loading={state === 'loading'}/>{state === 'loading' ? <p className="admin-loading">Đang tải tài khoản…</p> : rows.length ? <div className="admin-table-wrap"><table className="admin-table"><thead><tr><th>ID</th><th>Tài khoản</th><th>Vai trò</th><th>Trạng thái</th><th>Thao tác</th></tr></thead><tbody>{rows.map(row => <tr key={row.id}><td>#{row.id}</td><td><b>{row.name}</b><small>{row.email}</small></td><td>{statusLabel(row.role)}</td><td><span className={`status status-${row.status.toLowerCase()}`}>{statusLabel(row.status)}</span></td><td><button onClick={() => changeStatus(row)}>{row.status === 'SUSPENDED' ? 'Kích hoạt' : 'Tạm ngưng'}</button></td></tr>)}</tbody></table></div> : <div className="empty-table"><b>Chưa có tài khoản</b></div>}</>
}

function OrdersPanel() {
  const [rows, setRows] = useState<Order[]>([]); const [state, setState] = useState<'loading' | 'ready' | 'error'>('loading'); const [error, setError] = useState('')
  const load = useCallback(() => { setState('loading'); graphql<{ adminOrders: Order[] }>(`query AdminOrders { adminOrders(filter: { pagination: { skip: 0, take: 100 } }) { ${orderFields} } }`).then(r => { setRows(r.adminOrders); setState('ready') }).catch(e => { setError(e instanceof Error ? e.message : 'GraphQL không phản hồi.'); setState('error') }) }, [])
  useEffect(load, [load])
  async function cancel(row: Order) { const reason = window.prompt(`Lý do hủy đơn #${row.id}`); if (!reason?.trim()) return; try { await graphql(`mutation CancelOrder($id: Int!, $reason: String!) { adminCancelOrder(orderId: $id, reason: $reason) { ${orderFields} } }`, { id: row.id, reason }); load() } catch (e) { window.alert(e instanceof Error ? e.message : 'Không thể hủy đơn.') } }
  if (state === 'error') return <ErrorBox message={error} onRetry={load}/>
  return <><Toolbar onRefresh={load} loading={state === 'loading'}/>{state === 'loading' ? <p className="admin-loading">Đang tải đơn hàng…</p> : rows.length ? <div className="admin-table-wrap"><table className="admin-table"><thead><tr><th>Đơn</th><th>Tài khoản</th><th>Tổng tiền</th><th>Trạng thái</th><th>Thanh toán</th><th>Ngày tạo</th><th></th></tr></thead><tbody>{rows.map(row => <tr key={row.id}><td>#{row.id}</td><td>#{row.accountId}</td><td>{vnd(row.totalPrice)}</td><td>{statusLabel(row.status)}</td><td>{statusLabel(row.paymentStatus)}</td><td>{formatDate(row.createdAt)}</td><td><button onClick={() => cancel(row)} disabled={['cancelled', 'completed'].includes(row.status.toLowerCase())}>Yêu cầu hủy</button></td></tr>)}</tbody></table></div> : <div className="empty-table"><b>Chưa có đơn hàng</b></div>}</>
}

function TransactionsPanel() {
  const [rows, setRows] = useState<Transaction[]>([]); const [state, setState] = useState<'loading' | 'ready' | 'error'>('loading'); const [error, setError] = useState('')
  const load = useCallback(() => { setState('loading'); graphql<{ adminTransactions: Transaction[] }>(`query AdminTransactions { adminTransactions(pagination: { skip: 0, take: 100 }) { ${transactionFields} } }`).then(r => { setRows(r.adminTransactions); setState('ready') }).catch(e => { setError(e instanceof Error ? e.message : 'GraphQL không phản hồi.'); setState('error') }) }, [])
  useEffect(load, [load])
  async function reconcile(row: Transaction) { try { await graphql(`mutation Reconcile($id: String!) { adminReconcileTransaction(paymentId: $id) { ${transactionFields} } }`, { id: row.paymentId }); load() } catch (e) { window.alert(e instanceof Error ? e.message : 'Đối soát thất bại.') } }
  if (state === 'error') return <ErrorBox message={error} onRetry={load}/>
  return <><Toolbar onRefresh={load} loading={state === 'loading'}/>{state === 'loading' ? <p className="admin-loading">Đang tải giao dịch…</p> : rows.length ? <div className="admin-table-wrap"><table className="admin-table"><thead><tr><th>Payment ID</th><th>Đơn</th><th>Giá trị</th><th>Đã quyết toán</th><th>Trạng thái</th><th></th></tr></thead><tbody>{rows.map(row => <tr key={row.paymentId}><td><small>{row.paymentId}</small></td><td>#{row.orderId}</td><td>{row.totalPrice.toLocaleString('vi-VN')} {row.currency}</td><td>{row.settledPrice.toLocaleString('vi-VN')} {row.currency}</td><td>{statusLabel(row.status)}</td><td><button onClick={() => reconcile(row)}>Đối soát</button></td></tr>)}</tbody></table></div> : <div className="empty-table"><b>Chưa có giao dịch</b></div>}</>
}

function InventoryPanel() {
  const [stocks, setStocks] = useState<Stock[]>([]); const [reservations, setReservations] = useState<Reservation[]>([]); const [state, setState] = useState<'loading' | 'ready' | 'error'>('loading'); const [error, setError] = useState('')
  const load = useCallback(() => { setState('loading'); Promise.all([graphql<{ adminLowStock: Stock[] }>(`query LowStock { adminLowStock(limit: 100) { ${stockFields} } }`), graphql<{ adminReservations: Reservation[] }>(`query Reservations { adminReservations(pagination: { skip: 0, take: 100 }) { ${reservationFields} } }`)]).then(([stock, reservations]) => { setStocks(stock.adminLowStock); setReservations(reservations.adminReservations); setState('ready') }).catch(e => { setError(e instanceof Error ? e.message : 'GraphQL không phản hồi.'); setState('error') }) }, [])
  useEffect(load, [load])
  async function adjust() { const productId = window.prompt('Product ID cần điều chỉnh'); const delta = window.prompt('Số lượng tăng/giảm'); const reason = window.prompt('Lý do điều chỉnh'); if (!productId || !delta || !reason) return; try { await graphql(`mutation Adjust($input: InventoryAdjustmentInput!) { adminAdjustInventory(input: $input) { ${stockFields} } }`, { input: { productId, delta: Number(delta), reorderLevel: 0, reason } }); load() } catch (e) { window.alert(e instanceof Error ? e.message : 'Điều chỉnh tồn kho thất bại.') } }
  if (state === 'error') return <ErrorBox message={error} onRetry={load}/>
  return <><div className="panel-head"><span></span><button className="primary" onClick={adjust}>Điều chỉnh tồn kho</button></div><Toolbar onRefresh={load} loading={state === 'loading'}/>{state === 'loading' ? <p className="admin-loading">Đang tải tồn kho…</p> : <div className="admin-split"><div><h3>Tồn thấp</h3>{stocks.length ? <div className="admin-table-wrap"><table className="admin-table"><thead><tr><th>Sản phẩm</th><th>Có sẵn</th><th>Mức đặt lại</th><th>Đã giữ</th></tr></thead><tbody>{stocks.map(row => <tr key={row.productId}><td>{row.productId}</td><td>{row.availableQuantity}</td><td>{row.reorderLevel}</td><td>{row.reservedQuantity}</td></tr>)}</tbody></table></div> : <div className="empty-table"><b>Không có sản phẩm tồn thấp</b></div>}</div><div><h3>Phiên giữ hàng</h3>{reservations.length ? <div className="admin-table-wrap"><table className="admin-table"><thead><tr><th>Phiên</th><th>Sản phẩm</th><th>SL</th><th>Trạng thái</th></tr></thead><tbody>{reservations.map(row => <tr key={row.reservationId}><td><small>{row.reservationId}</small></td><td>{row.productId}</td><td>{row.quantity}</td><td>{statusLabel(row.status)}</td></tr>)}</tbody></table></div> : <div className="empty-table"><b>Chưa có phiên giữ hàng</b></div>}</div></div>}</>
}

function CatalogPanel() {
  const [rows, setRows] = useState<AdminProduct[]>([]); const [state, setState] = useState<'loading' | 'ready' | 'error'>('loading'); const [error, setError] = useState(''); const [editing, setEditing] = useState<AdminProduct | null>(null); const [formOpen, setFormOpen] = useState(false); const [form, setForm] = useState({ name: '', description: '', price: '', categoryId: '', brand: '', sku: '', thumbnail: '', images: '', tags: '' }); const [saving, setSaving] = useState(false)
  const productFields = 'id name description price categoryId brand sku thumbnail images tags publishStatus moderationStatus moderationReason'
  const load = useCallback(() => { setState('loading'); graphql<{ adminProducts: AdminProduct[] }>(`query AdminProducts { adminProducts(pagination: { skip: 0, take: 100 }) { ${productFields} } }`).then(r => { setRows(r.adminProducts); setState('ready') }).catch(e => { setError(e instanceof Error ? e.message : 'GraphQL không phản hồi.'); setState('error') }) }, [])
  useEffect(load, [load])
  function start(product?: AdminProduct) { setEditing(product || null); setFormOpen(true); setForm(product ? { name: product.name, description: product.description, price: String(product.price), categoryId: product.categoryId, brand: product.brand, sku: product.sku, thumbnail: product.thumbnail, images: product.images.join('\n'), tags: product.tags.join(', ') } : { name: '', description: '', price: '', categoryId: '', brand: '', sku: '', thumbnail: '', images: '', tags: '' }) }
  async function save() { if (!form.name.trim() || !form.description.trim() || !form.price || !form.categoryId.trim() || !form.thumbnail.trim()) { window.alert('Vui lòng nhập tên, mô tả, giá, category ID và ảnh đại diện.'); return }; setSaving(true); try { const input = { id: editing?.id, name: form.name.trim(), description: form.description.trim(), price: Number(form.price), categoryId: form.categoryId.trim(), brand: form.brand.trim(), sku: form.sku.trim(), thumbnail: form.thumbnail.trim(), images: form.images.split(/\n|,/).map(x => x.trim()).filter(Boolean), tags: form.tags.split(',').map(x => x.trim()).filter(Boolean), publishStatus: editing?.publishStatus || 'draft', moderationStatus: editing?.moderationStatus || 'pending', moderationReason: editing?.moderationReason || '' }; await graphql(`mutation SaveProduct($input: AdminProductInput!) { ${editing ? 'adminUpdateProduct' : 'adminCreateProduct'}(input: $input) { ${productFields} } }`, { input }); setFormOpen(false); setEditing(null); load() } catch (e) { window.alert(e instanceof Error ? e.message : 'Lưu sản phẩm thất bại.') } finally { setSaving(false) } }
  async function remove(product: AdminProduct) { if (!window.confirm(`Xóa sản phẩm ${product.name}?`)) return; try { await graphql('mutation DeleteProduct($id: String!) { adminDeleteProduct(id: $id) }', { id: product.id }); load() } catch (e) { window.alert(e instanceof Error ? e.message : 'Xóa sản phẩm thất bại.') } }
  async function upload(product: AdminProduct, file: File) { try { await graphqlUpload(`mutation UploadProductMedia($productId: String!, $file: Upload!, $altText: String!, $sortOrder: Int!) { adminUploadProductMedia(productId: $productId, file: $file, altText: $altText, sortOrder: $sortOrder) { id } }`, { productId: product.id, file: null, altText: product.name, sortOrder: product.images.length }, file); load() } catch (e) { window.alert(e instanceof Error ? e.message : 'Upload hình ảnh thất bại.') } }
  async function publish(product: AdminProduct) { try { await graphql(`mutation ModerateProduct($input: ModerateProductInput!) { adminModerateProduct(input: $input) { id publishStatus moderationStatus } }`, { input: { productId: product.id, publishStatus: 'published', moderationStatus: 'approved', moderationReason: '' } }); load() } catch (e) { window.alert(e instanceof Error ? e.message : 'Đăng sản phẩm thất bại.') } }
  if (state === 'error') return <ErrorBox message={error} onRetry={load}/>
  return <><div className="panel-head"><div><b>Quản lý sản phẩm</b><p>Thay đổi tại đây được lưu vào Product service và hiển thị lại trên trang khách hàng sau lần tải dữ liệu kế tiếp.</p></div><button className="primary" onClick={() => start()}>Thêm sản phẩm</button></div>{formOpen ? <div className="admin-product-form"><h3>{editing ? 'Sửa sản phẩm' : 'Thêm sản phẩm'}</h3><div className="admin-form-grid">{(['name', 'description', 'price', 'categoryId', 'brand', 'sku', 'thumbnail', 'images', 'tags'] as const).map(field => <label key={field}>{field === 'description' ? 'Mô tả' : field === 'categoryId' ? 'Category ID' : field === 'images' ? 'Các ảnh phụ (mỗi URL một dòng)' : field === 'tags' ? 'Tags (phân tách bằng dấu phẩy)' : field}<input value={form[field]} onChange={event => setForm({ ...form, [field]: event.target.value })} type={field === 'price' ? 'number' : 'text'} /></label>)}</div><div className="panel-actions"><button className="primary" onClick={save} disabled={saving}>{saving ? 'Đang lưu…' : 'Lưu sản phẩm'}</button><button onClick={() => { setFormOpen(false); setEditing(null); setForm({ name: '', description: '', price: '', categoryId: '', brand: '', sku: '', thumbnail: '', images: '', tags: '' }) }}>Hủy</button></div></div> : null}<Toolbar onRefresh={load} loading={state === 'loading'}/>{state === 'loading' ? <p className="admin-loading">Đang tải sản phẩm…</p> : rows.length ? <div className="admin-table-wrap"><table className="admin-table"><thead><tr><th>Ảnh</th><th>Sản phẩm</th><th>Giá</th><th>Danh mục</th><th>Trạng thái</th><th>Thao tác</th></tr></thead><tbody>{rows.map(row => <tr key={row.id}><td>{row.thumbnail ? <img className="admin-thumb" src={row.thumbnail} alt=""/> : '—'}</td><td><b>{row.name}</b><small>{row.description}</small></td><td>{vnd(row.price)}</td><td>{row.categoryId || '—'}</td><td>{statusLabel(row.publishStatus)} / {statusLabel(row.moderationStatus)}</td><td><button onClick={() => start(row)}>Sửa</button><button onClick={() => remove(row)}>Xóa</button>{row.publishStatus !== 'published' && <button onClick={() => publish(row)}>Đăng</button>}<label className="admin-upload">Thêm ảnh<input type="file" accept="image/*" onChange={event => { const file = event.target.files?.[0]; if (file) upload(row, file); event.currentTarget.value = '' }}/></label></td></tr>)}</tbody></table></div> : <div className="empty-table"><b>Chưa có sản phẩm</b></div>}</>
}

function AuditPanel() {
  const [rows, setRows] = useState<AuditEvent[]>([]); const [state, setState] = useState<'loading' | 'ready' | 'error'>('loading'); const [error, setError] = useState('')
  const load = useCallback(() => { setState('loading'); graphql<{ adminAuditEvents: AuditEvent[] }>(`query Audit { adminAuditEvents(pagination: { skip: 0, take: 100 }) { ${auditFields} } }`).then(r => { setRows(r.adminAuditEvents); setState('ready') }).catch(e => { setError(e instanceof Error ? e.message : 'GraphQL không phản hồi.'); setState('error') }) }, [])
  useEffect(load, [load])
  if (state === 'error') return <ErrorBox message={error} onRetry={load}/>
  return <><Toolbar onRefresh={load} loading={state === 'loading'}/>{state === 'loading' ? <p className="admin-loading">Đang tải nhật ký…</p> : rows.length ? <div className="admin-table-wrap"><table className="admin-table"><thead><tr><th>Thời gian</th><th>Người thao tác</th><th>Hành động</th><th>Kết quả</th><th>Request ID</th></tr></thead><tbody>{rows.map(row => <tr key={row.eventId}><td>{formatDate(row.occurredAt)}</td><td>#{row.actorAccountId}</td><td>{statusLabel(row.action)}</td><td>{statusLabel(row.outcome)}</td><td><small>{row.requestId}</small></td></tr>)}</tbody></table></div> : <div className="empty-table"><b>Chưa có sự kiện audit</b></div>}</>
}

function DataPanel({ section }: { section: string }) {
  const title = names[section]
  const content = useMemo(() => ({ catalog: <CatalogPanel/>, orders: <OrdersPanel/>, transactions: <TransactionsPanel/>, inventory: <InventoryPanel/>, accounts: <AccountsPanel/>, audit: <AuditPanel/> }[section]), [section])
  return <section className="admin-panel"><div><h2>{title}</h2><p>Dữ liệu thật được tải qua GraphQL và thao tác sẽ được kiểm tra quyền ở máy chủ.</p></div>{content}</section>
}

export function AdminPage({ section }: { section: string }) {
  return <main className="admin page-enter"><aside><Link className="brand" to="/">GOSHOP<span>X</span></Link><p>ADMIN CONSOLE</p>{items.map(([path, label]) => <Link className={section === path ? 'active' : ''} key={path} to={path ? `/admin/${path}` : '/admin'}>{label}</Link>)}</aside><section className="admin-content"><header><div><p className="eyebrow">OPERATE / MONITOR</p><h1>{section ? names[section] : 'Tổng quan vận hành'}</h1></div><span className="admin-period">Khoảng thời gian: 30 ngày</span></header>{section ? <DataPanel section={section}/> : <Dashboard/>}</section></main>
}