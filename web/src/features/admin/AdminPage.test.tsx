import { render, screen, waitFor } from '@testing-library/react'
import { MemoryRouter } from 'react-router-dom'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { AdminPage } from './AdminPage'

afterEach(() => vi.restoreAllMocks())

describe('AdminPage', () => {
  it('loads account data through the admin GraphQL query', async () => {
    const fetchMock = vi.spyOn(globalThis, 'fetch').mockImplementation(async (_input, init) => {
      const body = JSON.parse(String(init?.body ?? '{}')) as { query?: string }
      if (body.query?.includes('AdminAccounts')) {
        return new Response(JSON.stringify({ data: { adminAccounts: [{ id: 7, name: 'Admin User', email: 'admin@example.test', role: 'ADMIN', status: 'ACTIVE' }] } }), { status: 200 })
      }
      return new Response(JSON.stringify({ data: {} }), { status: 200 })
    })

    render(<MemoryRouter><AdminPage section="accounts" /></MemoryRouter>)

    expect(await screen.findByText('admin@example.test')).toBeInTheDocument()
    expect(screen.getByText('Admin User')).toBeInTheDocument()
    await waitFor(() => expect(fetchMock).toHaveBeenCalled())
  })
})
