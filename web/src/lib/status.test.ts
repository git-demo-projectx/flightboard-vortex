import { describe, expect, it } from 'vitest'
import { expenseStatusChip, healthChip, statusChip } from './status'
import { colors } from '../theme'

describe('expenseStatusChip', () => {
  it.each(['Pending', 'Approved', 'Rejected'] as const)('labels %s', (status) => {
    expect(expenseStatusChip(status).label).toBe(status)
  })

  it.each([
    ['Pending', colors.reviewBg, colors.reviewText],
    ['Approved', '#DDF3E8', colors.success],
    ['Rejected', '#F6D2BC', colors.alertText],
  ] as const)('uses the expense colours for %s', (status, bg, fg) => {
    expect(expenseStatusChip(status)).toMatchObject({ bg, fg })
  })
})

describe('statusChip', () => {
  it('shows Published in green', () => {
    expect(statusChip('Published')).toMatchObject({ label: 'Published', bg: '#038350' })
  })

  it.each(['Draft', 'Assigned', 'Work Done', 'Approved', 'Paid', 'Cancelled'] as const)(
    'labels %s with its own name',
    (status) => {
      expect(statusChip(status).label).toBe(status)
    },
  )
})

describe('healthChip', () => {
  it.each(['On Track', 'Review', 'Alert'] as const)('labels %s', (health) => {
    expect(healthChip(health).label).toBe(health)
  })

  it('uses the alert colours for Alert', () => {
    expect(healthChip('Alert').bg).toBe('#F6D2BC')
  })
})
