export function LoadingGrid() {
  return <div className="grid skeleton" aria-label="Đang tải sản phẩm">{Array.from({ length: 8 }, (_, index) => <i key={index}/>)}</div>
}

export function ErrorState({ message, onRetry }: { message: string; onRetry: () => void }) {
  return <main className="state"><h1>Danh mục đang tạm gián đoạn</h1><p>{message}</p><button onClick={onRetry}>Thử lại</button></main>
}
