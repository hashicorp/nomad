import s from './style.module.css'

export default function EcosystemIntegrationGroup({ heading, cards }) {
  return (
    <div className={s.ecosystemIntegrationGroup}>
      <h3 className="g-type-display-3">{heading}</h3>
      <ul className={s.companyGrid}>
        {cards.map((card) => {
          return <li key={card.companyName}>{card}</li>
        })}
      </ul>
    </div>
  )
}
