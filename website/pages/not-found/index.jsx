import s from './style.module.css'
import Link from 'next/link'
import { useEffect } from 'react'

export default function NotFound() {
  useEffect(() => {
    if (
      typeof window !== 'undefined' &&
      typeof window?.analytics?.track === 'function' &&
      typeof window?.document?.referrer === 'string' &&
      typeof window?.location?.href === 'string'
    )
      window.analytics.track(window.location.href, {
        category: '404 Response',
        label: window.document.referrer || 'No Referrer',
      })
  }, [])

  return (
    <div className={s.root}>
      <h1 className="g-type-display-1">Page Not Found</h1>
      <p>We're sorry but we can't find the page you're looking for.</p>
      <p>
        <Link href="/">
          <a>Back to Home</a>
        </Link>
      </p>
    </div>
  )
}
