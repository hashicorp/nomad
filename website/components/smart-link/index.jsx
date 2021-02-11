import Link from 'next/link'

export default function SmartLink({
  href,
  replace,
  scroll,
  shallow,
  passHref,
  children,
  external,
  ...props
}) {
  const isAbsoluteUrl = new RegExp('^(?:[a-z]+:)?//', 'i')

  if (!isAbsoluteUrl.test(href)) {
    return (
      <Link
        href={href}
        replace={replace}
        scroll={scroll}
        shallow={shallow}
        passHref={passHref}
      >
        <a {...props}>{children}</a>
      </Link>
    )
  } else {
    return (
      <a
        href={href}
        rel="noopener noreferrer"
        target={external ? '_blank' : '_self'}
        {...props}
      >
        {children}
      </a>
    )
  }
}
