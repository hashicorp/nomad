import Button from '@hashicorp/react-button'
import svgGithub from './icons/github.svg.js'
import GithubStarsLink from './github-stars-link/index.js'

function CtaLinks(props) {
  const { links, product, isInDropdown, hideGithubStars } = props
  return (
    <div className={`cta-links ${isInDropdown ? 'is-in-dropdown' : ''}`}>
      {links.map((link, stableIdx) => {
        const textKey = link.text.toLowerCase()
        const isDownload = textKey === 'download'
        const isGithub = textKey === 'github'
        if (isGithub && !isInDropdown)
          return (
            <GithubStarsLink
              // eslint-disable-next-line react/no-array-index-key
              key={stableIdx}
              url={link.url}
              hideGithubStars={hideGithubStars}
            />
          )
        const isLastButton = stableIdx === links.length - 1
        const iconDownload = {
          isAnimated: true,
          position: 'left',
        }
        const iconGithub = {
          svg: svgGithub,
          position: 'left',
        }
        return (
          <Button
            // eslint-disable-next-line react/no-array-index-key
            key={stableIdx}
            size="small"
            title={link.text}
            url={link.url}
            icon={isDownload ? iconDownload : isGithub ? iconGithub : undefined}
            theme={{
              brand: product,
              variant: isLastButton ? 'primary' : 'secondary',
            }}
            linkType={isDownload ? 'download' : undefined}
          />
        )
      })}
    </div>
  )
}

export default CtaLinks
