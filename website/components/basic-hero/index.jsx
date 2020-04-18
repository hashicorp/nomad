import Button from '@hashicorp/react-button'

export default function BasicHero({ heading, content, links }) {
  return (
    <div className="g-basic-hero">
      <div className="g-grid-container">
        <h1 className="g-type-display-1">{heading}</h1>
        <p className="g-type-body-large">{content}</p>
        {links && (
          <div className="links">
            {links.map((link, stableIdx) => {
              const buttonVariant = stableIdx === 0 ? 'primary' : 'secondary'
              const linkType = link.type || 'inbound'
              return (
                <Button
                  // eslint-disable-next-line react/no-array-index-key
                  key={stableIdx}
                  linkType={linkType}
                  theme={{
                    variant: buttonVariant,
                    brand: 'nomad',
                    background: 'light'
                  }}
                  title={link.text}
                  url={link.url}
                />
              )
            })}
          </div>
        )}
      </div>
    </div>
  )
}
