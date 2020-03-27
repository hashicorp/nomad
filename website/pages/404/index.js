import Link from 'next/link'
import { useEffect } from 'react'

function FourOhFour() {
  useEffect(() => {
    if (
      typeof window === 'object' &&
      typeof window?.analytics?.track === 'function' &&
      typeof window?.document?.referrer === 'string' &&
      typeof window?.location?.href === 'string'
    )
      window.analytics.track({
        event: '404 Response',
        action: window.location.href,
        label: window.document.referrer
      })
  }, [])

  return (
    <header id="p-404">
      <h1>Page Not Found</h1>
      <p>
        We&apos;re sorry but we can&apos;t find the page you&apos;re looking
        for.
      </p>
      <p>
        <Link href="/">
          <a>Back to Home</a>
        </Link>
      </p>
    </header>
  )
}

export default FourOhFour
