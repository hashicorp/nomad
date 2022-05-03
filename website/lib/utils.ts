export const isInternalLink = (link: string): boolean => {
  if (
    link.startsWith('/') ||
    link.startsWith('#') ||
    link.startsWith('https://nomadproject.io') ||
    link.startsWith('https://www.nomadproject.io')
  ) {
    return true
  }
  return false
}