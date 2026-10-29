import Box from '@mui/material/Box'
import Typography from '@mui/material/Typography'
import { useExpenses } from '../../api/hooks'
import type { WorkOrder } from '../../api/types'
import { ExpenseStatusChip } from '../../components/Chips'
import { formatCents } from '../../lib/format'

export function ExpenseList({ workOrder }: { workOrder: WorkOrder }) {
  const { data, error, loading } = useExpenses(workOrder.id)

  let content
  if (loading) {
    content = <Typography>Loading expenses...</Typography>
  } else if (error) {
    content = <Typography>Could not load expenses: {error.message}</Typography>
  } else if (data?.length) {
    content = (
      <Box component="ul" sx={{ m: 0, p: 0, listStyle: 'none', display: 'grid', gap: 2 }}>
        {data.map((expense) => (
          <Box component="li" key={expense.id} sx={{ minWidth: 0, overflowWrap: 'anywhere' }}>
            <Typography>{expense.description}</Typography>
            <Box sx={{ display: 'flex', alignItems: 'center', gap: 1, flexWrap: 'wrap', mt: 0.5 }}>
              <Typography variant="body2" color="text.secondary">{expense.category}</Typography>
              <Typography variant="body2">{formatCents(expense.amountCents)}</Typography>
              <ExpenseStatusChip status={expense.status} />
            </Box>
            {expense.status === 'Rejected' && expense.reason && (
              <Typography variant="body2" sx={{ mt: 0.5 }}>Reason: {expense.reason}</Typography>
            )}
          </Box>
        ))}
      </Box>
    )
  } else {
    content = <Typography>No expenses yet.</Typography>
  }

  return (
    <Box component="section" aria-label="Expenses">
      {content}
    </Box>
  )
}
