import { useEffect, useState } from 'react'
import { Link, useSearchParams } from 'react-router-dom'
import { graphql } from '../../shared/api/graphql'
import { useAuth } from '../auth/AuthContext'

type Transaction = { paymentId: string; status: string; totalPrice: number; settledPrice: number; currency: string }

export function PaymentReturnPage() {
  const [params] = useSearchParams()
  const { authenticated } = useAuth()
  const orderId = params.get('orderId') || params.get('vnp_TxnRef')
  const [state, setState] = useState<'missing' | 'loading' | 'success' | 'pending' | 'failed' | 'error'>('loading')
  const [message, setMessage] = useState('Đang xác minh trạng thái thanh toán với GoshopX…')

  useEffect(() => {
    if (!authenticated) {
      setState('error')
      setMessage('Bạn cần đăng nhập để xác minh giao dịch của chính mình.')
      return
    }
    if (!orderId || !/^\d+$/.test(orderId)) {
      setState('missing')
      setMessage('Cổng thanh toán chưa trả về mã đơn hàng hợp lệ; chưa thể kết luận thanh toán thành công.')
      return
    }

    let cancelled = false
    let attempts = 0
    const load = async () => {
      try {
        const result = await graphql<{ myPaymentTransactions: Transaction[] }>(
          'query MyPaymentTransactions($orderId: Int!) { myPaymentTransactions(orderId: $orderId) { paymentId status totalPrice settledPrice currency } }',
          { orderId: Number(orderId) },
        )
        if (cancelled) return
        const transaction = result.myPaymentTransactions[0]
        const status = transaction?.status?.toLowerCase()
        if (status === 'success' || status === 'succeeded' || status === 'paid') {
          setState('success')
          setMessage('Backend đã xác nhận thanh toán thành công.')
        } else if (status === 'failed' || status === 'cancelled' || status === 'expired') {
          setState('failed')
          setMessage(`Thanh toán chưa thành công (trạng thái: ${transaction.status}).`)
        } else if (attempts < 4) {
          attempts += 1
          setState('pending')
          setMessage('Giao dịch đang chờ callback/webhook; hệ thống sẽ kiểm tra lại…')
          window.setTimeout(() => void load(), 2000)
        } else {
          setState('pending')
          setMessage('Chưa nhận được xác nhận cuối cùng từ payment backend. Không đánh dấu thành công.')
        }
      } catch (cause) {
        if (!cancelled) {
          setState('error')
          setMessage(cause instanceof Error ? cause.message : 'Không thể xác minh thanh toán.')
        }
      }
    }
    void load()
    return () => { cancelled = true }
  }, [authenticated, orderId])

  const title = state === 'success' ? 'Thanh toán thành công' : state === 'failed' ? 'Thanh toán thất bại' : state === 'pending' ? 'Đang chờ xác nhận' : 'Chưa thể xác minh thanh toán'
  return <main className="state checkout page-enter"><p className="eyebrow">PAYMENT STATUS</p><h1>{title}</h1><p>{message}</p><div className="state-actions"><Link className="button" to="/account/orders">Xem đơn hàng</Link><Link className="button secondary-action" to="/">Tiếp tục mua sắm</Link></div></main>
}
