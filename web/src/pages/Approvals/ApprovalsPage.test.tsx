import { act, fireEvent, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { api } from '../../api/client'
import type { WorkOrder } from '../../api/types'
import { formatPay, formatTime } from '../../lib/format'
import { makeWorkOrder } from '../../test/fixtures'
import { renderAt } from '../../test/render'
import { ApprovalsPage } from './ApprovalsPage'

vi.mock('../../api/client', () => ({
  api: { listWorkOrders: vi.fn(), setStatus: vi.fn() },
}))

const orders = [
  makeWorkOrder({ id: 9631820, title: 'Finished network work', status: 'Work Done' }),
  makeWorkOrder({ id: 9631818, title: 'Finished printer work', status: 'Work Done', checkOut: '2026-10-22T18:00:00Z' }),
  makeWorkOrder({ id: 9631819, title: 'Finished phone work', status: 'Work Done', provider: null }),
  makeWorkOrder({ id: 9631810, title: 'Assigned work', status: 'Assigned' }),
  makeWorkOrder({ id: 9631821, title: 'Approved work', status: 'Approved' }),
]

beforeEach(() => {
  vi.mocked(api.listWorkOrders).mockReset().mockResolvedValue(orders)
  vi.mocked(api.setStatus).mockReset().mockImplementation(async (id) =>
    makeWorkOrder({ id, status: 'Approved' }),
  )
})

describe('ApprovalsPage', () => {
  it('lists Work Done work orders by id with all selected', async () => {
    renderAt(<ApprovalsPage />)
    const table = await screen.findByRole('table', { name: 'Work Done work orders' })
    const rows = within(table).getAllByRole('row').slice(1)

    expect(rows.map((row) => within(row).getByRole('link').textContent)).toEqual(['9631818', '9631819', '9631820'])
    for (const checkbox of screen.getAllByRole('checkbox')) expect(checkbox).toBeChecked()
    expect(screen.getByRole('button', { name: 'Approve 3 work orders' })).toBeEnabled()
    expect(screen.queryByText('Assigned work')).not.toBeInTheDocument()
    expect(screen.queryByText('Approved work')).not.toBeInTheDocument()
    expect(screen.getByRole('link', { name: '9631818' })).toHaveAttribute('href', '/work-orders/9631818')
    expect(screen.getByRole('link', { name: 'Back to Flightboard' })).toHaveAttribute('href', '/')
    expect(within(rows[0]).getByText('Networking \u00b7 Acme Test Co')).toBeInTheDocument()
    expect(within(rows[0]).getByText('Alex Test')).toBeInTheDocument()
    expect(within(rows[0]).getByText(formatTime(orders[1].checkOut!))).toBeInTheDocument()
    expect(within(rows[0]).getByText(formatPay(orders[1].pay))).toBeInTheDocument()
    expect(within(rows[1]).getByText('Unassigned')).toBeInTheDocument()
    expect(within(rows[1]).getByText('--')).toBeInTheDocument()
  })

  it('updates the selection', async () => {
    renderAt(<ApprovalsPage />)
    await userEvent.click(await screen.findByRole('checkbox', { name: 'Select WO 9631819' }))

    expect(screen.getByText('2 of 3 selected')).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Approve 2 work orders' })).toBeEnabled()
    expect(screen.getByRole('checkbox', { name: 'Select all' })).toHaveAttribute('data-indeterminate', 'true')
    await userEvent.click(screen.getByRole('checkbox', { name: 'Select all' }))
    expect(screen.getByText('3 of 3 selected')).toBeInTheDocument()
    await userEvent.click(screen.getByRole('checkbox', { name: 'Select all' }))
    expect(screen.getByText('0 of 3 selected')).toBeInTheDocument()
    const button = screen.getByRole('button', { name: 'Approve 0 work orders' })
    expect(button).toBeDisabled()
    fireEvent.click(button)
    expect(api.setStatus).not.toHaveBeenCalled()
  })

  it('approves the selected work orders in id order', async () => {
    vi.mocked(api.listWorkOrders).mockResolvedValueOnce(orders).mockResolvedValueOnce([orders[2]])
    renderAt(<ApprovalsPage />)
    await userEvent.click(await screen.findByRole('checkbox', { name: 'Select WO 9631819' }))
    await userEvent.click(screen.getByRole('button', { name: 'Approve 2 work orders' }))

    expect(await screen.findByText('Approved 2 work orders')).toBeInTheDocument()
    expect(vi.mocked(api.setStatus).mock.calls).toEqual([[9631818, 'Approved'], [9631820, 'Approved']])
    await screen.findByText('1 of 1 selected')
    expect(api.listWorkOrders).toHaveBeenCalledTimes(2)
    expect(screen.queryByRole('link', { name: '9631818' })).not.toBeInTheDocument()
    expect(screen.queryByRole('link', { name: '9631820' })).not.toBeInTheDocument()
    expect(screen.getByRole('checkbox', { name: 'Select WO 9631819' })).toBeChecked()
    expect(screen.getByRole('alert')).toHaveTextContent('Approved 2 work orders')
  })

  it('reports work orders that were not approved', async () => {
    vi.mocked(api.setStatus).mockImplementation(async (id) => {
      if (id === 9631819) throw new Error('approve or reject the pending expenses first')
      return makeWorkOrder({ id, status: 'Approved' })
    })
    vi.mocked(api.listWorkOrders).mockResolvedValueOnce(orders).mockResolvedValue([orders[2]])
    renderAt(<ApprovalsPage />)
    await userEvent.click(await screen.findByRole('button', { name: 'Approve 3 work orders' }))

    await screen.findByText('1 of 1 selected')
    expect(screen.getByText('Approved 2 work orders')).toBeInTheDocument()
    expect(screen.getAllByRole('alert')).toHaveLength(2)
    expect(screen.getAllByRole('alert')[1]).toHaveTextContent('Could not approve: WO 9631819: approve or reject the pending expenses first')
    expect(vi.mocked(api.setStatus).mock.calls).toEqual([
      [9631818, 'Approved'], [9631819, 'Approved'], [9631820, 'Approved'],
    ])
    expect(api.listWorkOrders).toHaveBeenCalledTimes(2)

    let resolve!: (order: WorkOrder) => void
    vi.mocked(api.setStatus).mockReturnValueOnce(new Promise((done) => { resolve = done }))
    await userEvent.click(screen.getByRole('button', { name: 'Approve 1 work order' }))
    expect(screen.queryByRole('alert')).not.toBeInTheDocument()
    await act(async () => resolve(makeWorkOrder({ id: 9631819, status: 'Approved' })))
    expect(await screen.findByText('Approved 1 work order')).toBeInTheDocument()
  })

  it('disables the controls while approvals run', async () => {
    let resolve!: (order: WorkOrder) => void
    vi.mocked(api.setStatus).mockReturnValueOnce(new Promise((done) => { resolve = done }))
    renderAt(<ApprovalsPage />)
    await userEvent.click(await screen.findByRole('button', { name: 'Approve 3 work orders' }))

    const button = screen.getByRole('button', { name: 'Approve 3 work orders' })
    expect(button).toBeDisabled()
    for (const checkbox of screen.getAllByRole('checkbox')) expect(checkbox).toBeDisabled()
    fireEvent.click(button)
    expect(api.setStatus).toHaveBeenCalledExactlyOnceWith(9631818, 'Approved')
    expect(api.listWorkOrders).toHaveBeenCalledTimes(1)

    await act(async () => resolve(makeWorkOrder({ id: 9631818, status: 'Approved' })))
    await screen.findByText('Approved 3 work orders')
    await waitFor(() => expect(api.listWorkOrders).toHaveBeenCalledTimes(2))
    expect(api.setStatus).toHaveBeenCalledTimes(3)
  })

  it('reports every failure and reloads even when none succeeded', async () => {
    vi.mocked(api.setStatus).mockRejectedValue('offline')
    renderAt(<ApprovalsPage />)
    await userEvent.click(await screen.findByRole('button', { name: 'Approve 3 work orders' }))

    await waitFor(() => expect(api.listWorkOrders).toHaveBeenCalledTimes(2))
    const alert = screen.getByRole('alert')
    expect(alert).toHaveTextContent('Could not approve:')
    for (const id of [9631818, 9631819, 9631820]) {
      expect(within(alert).getByText(`WO ${id}: offline`)).toBeInTheDocument()
    }
    expect(screen.queryByText(/^Approved \d/)).not.toBeInTheDocument()
    await screen.findByText('3 of 3 selected')
  })

  it('shows an empty state', async () => {
    vi.mocked(api.listWorkOrders).mockResolvedValue([orders[3]])
    renderAt(<ApprovalsPage />)
    expect(await screen.findByText('No work orders are waiting for approval.')).toBeInTheDocument()
  })

  it('shows a load error', async () => {
    vi.mocked(api.listWorkOrders).mockRejectedValue(new Error('boom'))
    renderAt(<ApprovalsPage />)
    expect(await screen.findByRole('alert')).toHaveTextContent('Could not load work orders: boom')
  })

  it('shows a progress indicator while loading', async () => {
    let resolve!: (list: WorkOrder[]) => void
    vi.mocked(api.listWorkOrders).mockReturnValueOnce(new Promise((done) => { resolve = done }))
    renderAt(<ApprovalsPage />)
    expect(screen.getByRole('progressbar')).toBeInTheDocument()
    expect(screen.queryByRole('table')).not.toBeInTheDocument()
    await act(async () => resolve([]))
    expect(screen.queryByRole('progressbar')).not.toBeInTheDocument()
  })
})
