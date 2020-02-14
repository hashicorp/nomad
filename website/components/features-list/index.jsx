export default function FeaturesList({ title, items }) {
  return (
    <div className="g-features-list g-grid-container">
      <h2 className="g-type-display-2">{title}</h2>
      <div className="items-container">
        {items.map(({ title, content, icon }) => (
          <div key={title} className="item">
            <div className="item-icon">
              <img src={icon} alt={title} />
            </div>
            <div className="content">
              <h4 className="g-type-display-4">{title}</h4>
              <p className="g-type-body-small">{content}</p>
            </div>
          </div>
        ))}
      </div>
    </div>
  )
}
