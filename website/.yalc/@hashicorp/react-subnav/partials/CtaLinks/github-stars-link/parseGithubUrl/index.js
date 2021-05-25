/*
 * Given:
 * urlString (string) valid URL string
 * Return:
 * { org, repo }, where the url is ~= www.github.com/{org}/{repo}
 * or false otherwise (eg if not a valid URL, or if not a GitHub url)
 */
function parseGithubUrl(urlString) {
  try {
    const urlObj = new URL(urlString)
    if (urlObj.hostname !== 'www.github.com') return false
    const parts = urlObj.pathname.split('/').filter(Boolean)
    const org = parts[0]
    const repo = parts[1]
    return { org, repo }
  } catch (err) {
    console.warn(
      'Warning! An invalid URL has probably been supplied to the GitHub ctaLink in <Subnav />. The corresponding error:'
    )
    console.warn(err)
    return false
  }
}

export default parseGithubUrl
