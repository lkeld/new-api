/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.

For commercial licensing, please contact support@quantumnous.com
*/
import { useState, useMemo } from 'react'
import { type Table } from '@tanstack/react-table'
import { Trash2 } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { Button } from '@/components/ui/button'
import {
  Tooltip,
  TooltipContent,
  TooltipTrigger,
} from '@/components/ui/tooltip'
import { ConfirmDialog } from '@/components/confirm-dialog'
import { CopyButton } from '@/components/copy-button'
import { DataTableBulkActions as BulkActionsToolbar } from '@/components/data-table'
import { deleteInvalidInviteCodes } from '../api'
import { type InviteCode } from '../types'
import { useInviteCodes } from './invite-codes-provider'

type DataTableBulkActionsProps<TData> = {
  table: Table<TData>
}

export function DataTableBulkActions<TData>({
  table,
}: DataTableBulkActionsProps<TData>) {
  const { t } = useTranslation()
  const { triggerRefresh } = useInviteCodes()
  const [showDeleteInvalidConfirm, setShowDeleteInvalidConfirm] =
    useState(false)
  const [isDeleting, setIsDeleting] = useState(false)
  const selectedRows = table.getFilteredSelectedRowModel().rows

  const contentToCopy = useMemo(() => {
    const selectedCodes = selectedRows.map((row) => {
      const inviteCode = row.original as InviteCode
      return `${inviteCode.name}\t${inviteCode.code}`
    })
    return selectedCodes.join('\n')
  }, [selectedRows])

  const handleDeleteInvalid = async () => {
    setIsDeleting(true)
    try {
      const result = await deleteInvalidInviteCodes()

      if (result.success) {
        const count = result.data || 0
        toast.success(
          t('Successfully deleted {{count}} invalid invite codes', {
            count,
          })
        )
        table.resetRowSelection()
        triggerRefresh()
        setShowDeleteInvalidConfirm(false)
      }
    } finally {
      setIsDeleting(false)
    }
  }

  return (
    <>
      <BulkActionsToolbar table={table} entityName={t('invite code')}>
        <CopyButton
          value={contentToCopy}
          variant='outline'
          size='icon'
          className='size-8'
          tooltip={t('Copy selected codes')}
          successTooltip={t('Codes copied!')}
          aria-label={t('Copy selected codes')}
        />

        <Tooltip>
          <TooltipTrigger
            render={
              <Button
                variant='destructive'
                size='icon'
                onClick={() => setShowDeleteInvalidConfirm(true)}
                className='size-8'
                aria-label={t('Delete invalid invite codes')}
                title={t('Delete invalid invite codes')}
              />
            }
          >
            <Trash2 />
            <span className='sr-only'>{t('Delete invalid codes')}</span>
          </TooltipTrigger>
          <TooltipContent>
            <p>{t('Delete invalid codes (exhausted/disabled/expired)')}</p>
          </TooltipContent>
        </Tooltip>
      </BulkActionsToolbar>

      <ConfirmDialog
        destructive
        open={showDeleteInvalidConfirm}
        onOpenChange={setShowDeleteInvalidConfirm}
        handleConfirm={handleDeleteInvalid}
        isLoading={isDeleting}
        className='max-w-md'
        title={t('Delete Invalid Invite Codes?')}
        desc={
          <>
            {t('This will delete all')} <strong>{t('exhausted')}</strong>,{' '}
            <strong>{t('disabled')}</strong>
            {t(', and')} <strong>{t('expired')}</strong>{' '}
            {t('invite codes.')}
            <br />
            {t('This action cannot be undone.')}
          </>
        }
        confirmText={t('Delete Invalid')}
      />
    </>
  )
}
