import React, { useEffect, useState } from 'react'
import fetch from 'isomorphic-unfetch'
import formatStarCount from './formatStarCount/index.js'
import parseGithubUrl from './parseGithubUrl/index.js'
import StarIcon from '../icons/star'
import GithubIcon from '../icons/github-logo'

function GithubStarsButton(props) {
  const { url, hideGithubStars } = props
  const [starCount, setStarcount] = useState(-1)

  useEffect(() => {
    if (hideGithubStars) return setStarcount(0)
    const { org, repo } = parseGithubUrl(url)
    if (!org || !repo) return setStarcount(0)
    const githubApiUrl = `https://api.github.com/repos/${org}/${repo}`
    fetch(githubApiUrl)
      .then((response) => {
        response.json().then((data) => {
          // Github's rate limit for unauthenticated requests is 60 per hour
          // When the limit is hit, data.stargazers_count is undefined,
          // and setStarcount falls back to not showing the star count
          setStarcount(data.stargazers_count)
          // Warn if this limit is hit, to avoid otherwise confusing behavior
          //  We're still using the response to provide a documentation link
          if (!data.stargazers_count) {
            const { headers } = response
            if (headers.get('x-ratelimit-remaining') === '0') {
              const resetAtSeconds = parseInt(headers.get('x-ratelimit-reset'))
              const resetDate = new Date(resetAtSeconds * 1000)
              const rateLimit = headers.get('x-ratelimit-limit')
              console.warn(
                `⭐ Stargazers count could not be fetched. Rate limit exceeded for unauthenticated GitHub API. Limit will be reset to ${rateLimit} at ${resetDate}. See ${data.documentation_url} for more details.`
              )
            } else {
              console.warn(
                `Request for stargazers was successful, but the returned value was undefined or falsy. This might be because the repo has no stars, or it might be a different issue.`
              )
            }
          }
        })
      })
      .catch((err) => {
        setStarcount(0)
        console.warn(JSON.stringify(err, null, 2))
      })
  }, [url])

  const isLoadingStarcount = starCount === -1
  const isFailedStarcount = formatStarCount(starCount) === false
  const showStarcount =
    !hideGithubStars && (isLoadingStarcount || !isFailedStarcount)
  return (
    <a href={url} className="github-button">
      <span className="github">
        <GithubIcon />
        <span className="hidden-text">GitHub</span>
      </span>
      {showStarcount && (
        <span className="stars">
          <StarIcon />
          <span className="g-type-body-small-strong" data-testid="github-stars">
            {formatStarCount(starCount) || <span>&mdash;</span>}
          </span>
        </span>
      )}
    </a>
  )
}

export default GithubStarsButton
