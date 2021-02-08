import s from './style.module.css'
import Image from '@hashicorp/react-image'
import Button from '@hashicorp/react-image'

export default function ComparisonCallouts({
  heading,
  details,
  detailCta,
  items,
}) {
  return (
    <div className={s.comparisonCallouts}>
      <div className={s.content}>
        <div className={s.description}>
          <h2 className="g-type-display-2">{heading}</h2>
          <p className={s.details}>
            {details} <a href={detailCta.url}>{detailCta.title}</a>
          </p>
        </div>

        <div className={s.comparisonItems}>
          {items.map((item) => (
            <ComparisonItem key={item.title} {...item} />
          ))}
        </div>
      </div>
    </div>
  )
}

function ComparisonItem({ imageUrl, title, description, link }) {
  return (
    <div className={s.item}>
      <Image url={imageUrl} />
      <h4 className="g-type-display-4">{title}</h4>
      <p className="g-type-body">{description}</p>
      <Button
        url={link.url}
        title={link.text}
        linkType={link.type}
        theme={{ variant: 'tertiary', brand: 'nomad' }}
      />
    </div>
  )
}
