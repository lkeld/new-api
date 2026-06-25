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
import { useQuery } from '@tanstack/react-query'
import { KeyRound } from 'lucide-react'
import { Skeleton } from '@/components/ui/skeleton'
import { getKeyStatus } from '@/features/dashboard/api'

interface KeyStatusQuota {
  expire_at?: string
  daily_calls?: number
  daily_limit?: number
}

interface KeyStatusData {
  quota?: KeyStatusQuota | null
  allocated_quota?: number
  quota_per_unit?: number
  upstream_ok?: boolean
}

// The upstream quota API reports expire_at in Beijing time (UTC+8); interpret it
// as such so the countdown is correct regardless of the viewer's timezone.
function parseBeijingDateTime(value?: string): number | null {
  if (!value) return null
  const match = value
    .trim()
    .match(/^(\d{4})-(\d{2})-(\d{2})[ T](\d{2}):(\d{2}):(\d{2})/)
  if (!match) {
    const fallback = Date.parse(value)
    return Number.isNaN(fallback) ? null : fallback
  }
  const [, y, mo, d, h, mi, s] = match
  // Beijing (UTC+8) wall-clock components -> UTC epoch millis.
  return (
    Date.UTC(
      Number(y),
      Number(mo) - 1,
      Number(d),
      Number(h),
      Number(mi),
      Number(s)
    ) -
    8 * 3600 * 1000
  )
}

function formatExpiresIn(expireAt?: string): string {
  const target = parseBeijingDateTime(expireAt)
  if (target === null) return '—'
  const remaining = target - Date.now()
  if (remaining <= 0) return 'Expired'
  const totalMinutes = Math.floor(remaining / 60000)
  const hours = Math.floor(totalMinutes / 60)
  const minutes = totalMinutes % 60
  return `Expires in ${hours}h ${minutes}m`
}

export function KeyStatusPanel() {
  const { data, isLoading } = useQuery({
    queryKey: ['dashboard', 'overview', 'key-status'],
    queryFn: getKeyStatus,
    staleTime: 60000,
    refetchInterval: 60000,
  })

  const status: KeyStatusData | null | undefined = data?.data

  return (
    <div className='bg-card h-full rounded-2xl border p-4 shadow-xs sm:p-5'>
      <div className='flex items-center gap-2'>
        <span className='flex size-8 items-center justify-center rounded-lg bg-red-500/10 text-red-500'>
          <KeyRound className='size-4' />
        </span>
        <h3 className='text-sm font-medium'>上游密钥 / Key Status</h3>
      </div>

      {isLoading ? (
        <div className='mt-4 space-y-3'>
          <Skeleton className='h-8 w-40' />
          <Skeleton className='h-4 w-32' />
          <Skeleton className='h-4 w-32' />
        </div>
      ) : !status || !status.quota ? (
        <p className='text-muted-foreground mt-4 text-sm'>no upstream key</p>
      ) : (
        <div className='mt-4 space-y-3'>
          <div className='text-2xl font-semibold tracking-tight text-red-500'>
            {formatExpiresIn(status.quota.expire_at)}
          </div>
          <div className='flex items-center justify-between text-sm'>
            <span className='text-muted-foreground'>Today</span>
            <span className='font-medium tabular-nums'>
              {status.quota.daily_calls ?? 0} / {status.quota.daily_limit ?? 0}
            </span>
          </div>
          <div className='flex items-center justify-between text-sm'>
            <span className='text-muted-foreground'>Allocated</span>
            <span className='font-medium tabular-nums'>
              {'$' +
                (
                  (status.allocated_quota ?? 0) / (status.quota_per_unit || 1)
                ).toFixed(2)}
            </span>
          </div>
        </div>
      )}
    </div>
  )
}
