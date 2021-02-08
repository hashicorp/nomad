import s from './style.module.css'

export default function ComparisonCallouts({
  heading,
  details,
  detailCta,
  itemOne,
  itemTwo,
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
          <div className={s.itemOne}>{itemOne}</div>
          <div className={s.itemTwo}>{itemTwo}</div>
        </div>
      </div>
    </div>
  )
}
