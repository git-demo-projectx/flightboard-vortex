import DeleteOutline from '@mui/icons-material/DeleteOutline'
import Alert from '@mui/material/Alert'
import Box from '@mui/material/Box'
import Button from '@mui/material/Button'
import Dialog from '@mui/material/Dialog'
import DialogActions from '@mui/material/DialogActions'
import DialogContent from '@mui/material/DialogContent'
import DialogTitle from '@mui/material/DialogTitle'
import IconButton from '@mui/material/IconButton'
import MenuItem from '@mui/material/MenuItem'
import TextField from '@mui/material/TextField'
import Tooltip from '@mui/material/Tooltip'
import Typography from '@mui/material/Typography'
import { useState } from 'react'
import { api } from '../../api/client'
import type { AsyncState } from '../../api/hooks'
import type { Adjustment, AdjustmentKind, WorkOrder } from '../../api/types'
import { formatMoney } from '../../lib/format'
import { canAdjustPay } from '../../lib/status'

export function AdjustmentsPanel({ wo, adjustments }: Readonly<{
  wo: WorkOrder
  adjustments: AsyncState<Adjustment[]>
}>) {
  const [open, setOpen] = useState(false)
  const [kind, setKind] = useState<AdjustmentKind>('Bonus')
  const [reason, setReason] = useState('')
  const [amount, setAmount] = useState('')
  const [busy, setBusy] = useState(false)
  const [addError, setAddError] = useState<string | null>(null)
  const [removeError, setRemoveError] = useState<string | null>(null)
  const rows = adjustments.data ?? []
  const editable = canAdjustPay(wo.status)
  const atLimit = rows.length >= 5
  const trimmedReason = reason.trim()
  const dollars = Number(amount)
  const valid = trimmedReason.length >= 1 && trimmedReason.length <= 120
    && Number.isFinite(dollars) && dollars >= 0.01 && dollars <= 1000

  function openDialog() {
    setKind('Bonus')
    setReason('')
    setAmount('')
    setAddError(null)
    setOpen(true)
  }

  async function save() {
    if (!valid || busy || !editable || atLimit) return
    setBusy(true)
    setAddError(null)
    try {
      const added = await api.addAdjustment(wo.id, {
        kind, reason: trimmedReason, amountCents: Math.round(dollars * 100),
      })
      adjustments.setData([...rows, added])
      setOpen(false)
    } catch (error) {
      setAddError((error as Error).message)
    } finally {
      setBusy(false)
    }
  }

  async function remove(id: string) {
    if (busy || !editable) return
    setBusy(true)
    setRemoveError(null)
    try {
      await api.removeAdjustment(wo.id, id)
      adjustments.setData(rows.filter((row) => row.id !== id))
    } catch (error) {
      setRemoveError((error as Error).message)
    } finally {
      setBusy(false)
    }
  }

  if (adjustments.loading) {
    return <Typography color="text.secondary">Loading adjustments...</Typography>
  }
  if (adjustments.error) {
    return <Alert severity="warning">Adjustments unavailable: {adjustments.error.message}</Alert>
  }

  return (
    <>
      {rows.length === 0 && <Typography color="text.secondary">No adjustments.</Typography>}
      {rows.map((row) => (
        <Box key={row.id} sx={{ display: 'flex', alignItems: 'center', gap: 1, py: 0.5 }}>
          <Box sx={{ flex: 1, minWidth: 0, overflowWrap: 'anywhere' }}>
            <Typography sx={{ fontWeight: 600 }}>{row.kind}</Typography>
            <Typography>{row.reason}</Typography>
          </Box>
          <Typography sx={{ whiteSpace: 'nowrap' }}>
            {row.kind === 'Bonus' ? '+' : '-'}{formatMoney(row.amountCents / 100)}
          </Typography>
          {editable && (
            <Tooltip title={`Remove adjustment ${row.reason}`}>
              <span>
                <IconButton aria-label={`Remove adjustment ${row.reason}`} disabled={busy} onClick={() => remove(row.id)}>
                  <DeleteOutline />
                </IconButton>
              </span>
            </Tooltip>
          )}
        </Box>
      ))}
      {removeError && <Alert severity="error" sx={{ mt: 1 }}>{removeError}</Alert>}
      {atLimit && <Typography color="text.secondary" sx={{ mt: 1 }}>A work order can have at most 5 adjustments.</Typography>}
      {editable && (
        <Button variant="outlined" disabled={atLimit || busy} onClick={openDialog} sx={{ mt: 2 }}>Add adjustment</Button>
      )}
      <Dialog open={open && editable} onClose={() => { if (!busy) setOpen(false) }} fullWidth maxWidth="sm" aria-labelledby="add-adjustment-title">
        <DialogTitle id="add-adjustment-title">Add adjustment</DialogTitle>
        <DialogContent>
          <Box sx={{ display: 'grid', gap: 2, pt: 1 }}>
            <TextField select label="Kind" value={kind} disabled={busy} onChange={(event) => setKind(event.target.value as AdjustmentKind)}>
              <MenuItem value="Bonus">Bonus</MenuItem>
              <MenuItem value="Deduction">Deduction</MenuItem>
            </TextField>
            <TextField label="Reason" value={reason} disabled={busy} onChange={(event) => setReason(event.target.value)} />
            <TextField label="Amount ($)" type="number" value={amount} disabled={busy} inputProps={{ min: 0.01, max: 1000, step: 0.01 }} onChange={(event) => setAmount(event.target.value)} />
            {addError && <Alert severity="error">{addError}</Alert>}
          </Box>
        </DialogContent>
        <DialogActions>
          <Button disabled={busy} onClick={() => setOpen(false)}>Cancel</Button>
          <Button variant="contained" disabled={!valid || busy || !editable || atLimit} onClick={save}>Save</Button>
        </DialogActions>
      </Dialog>
    </>
  )
}
