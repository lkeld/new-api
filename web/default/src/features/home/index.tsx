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
import { useEffect, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { useAuthStore } from '@/stores/auth-store'
import { Markdown } from '@/components/ui/markdown'
import { PublicLayout } from '@/components/layout'
import { Footer } from '@/components/layout/components/footer'
import { CTA, Features, Hero, HowItWorks, Stats } from './components'
import { useHomePageContent } from './hooks'

export function Home() {
  const { t } = useTranslation()
  const { auth } = useAuthStore()
  const isAuthenticated = !!auth.user
  const { content, isLoaded, isUrl } = useHomePageContent()

  // A self-contained HTML landing reports its own scaled height via postMessage so the iframe can
  // grow to fit it — this keeps the new-api header/nav above it instead of covering the page.
  const [landingHeight, setLandingHeight] = useState<number | null>(null)
  useEffect(() => {
    function onMsg(e: MessageEvent) {
      const h = (e.data as { jnHeight?: number } | null)?.jnHeight
      if (typeof h === 'number' && h > 0) setLandingHeight(Math.ceil(h))
    }
    window.addEventListener('message', onMsg)
    return () => window.removeEventListener('message', onMsg)
  }, [])

  if (!isLoaded) {
    return (
      <PublicLayout showMainContainer={false}>
        <main className='flex min-h-screen items-center justify-center'>
          <div className='text-muted-foreground'>{t('Loading...')}</div>
        </main>
      </PublicLayout>
    )
  }

  if (content) {
    // A self-contained HTML landing (our 江南皮革厂 page) renders in an isolated iframe BELOW the
    // public header, so the original nav + login/register stay reachable. It posts its scaled
    // height and the iframe grows to fit. <base target="_top"> (injected at deploy) makes its
    // links navigate the top window.
    const looksLikeHtml =
      !isUrl && /^\s*<(?:!doctype|html|body|div|section|main|header|style)/i.test(content)
    if (looksLikeHtml) {
      return (
        <PublicLayout showMainContainer={false}>
          <iframe
            srcDoc={content}
            title={t('Custom Home Page')}
            scrolling='no'
            className='block w-full border-none'
            style={{ height: landingHeight ? `${landingHeight}px` : '100vh' }}
          />
        </PublicLayout>
      )
    }
    return (
      <PublicLayout showMainContainer={false}>
        <main className='overflow-x-hidden'>
          {isUrl ? (
            <iframe
              src={content}
              className='h-screen w-full border-none'
              title={t('Custom Home Page')}
            />
          ) : (
            <div className='container mx-auto py-8'>
              <Markdown className='custom-home-content'>{content}</Markdown>
            </div>
          )}
        </main>
      </PublicLayout>
    )
  }

  return (
    <PublicLayout showMainContainer={false}>
      <Hero isAuthenticated={isAuthenticated} />
      <Stats />
      <Features />
      <HowItWorks />
      <CTA isAuthenticated={isAuthenticated} />
      <Footer />
    </PublicLayout>
  )
}
