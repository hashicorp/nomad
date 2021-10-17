export default function FeaturesList({ title, items, intro }) {
  return (
    <div className="g-features-list g-grid-container">
      <h2 className="g-type-display-2">{title}</h2>
      <div
        className="intro-container"
        dangerouslySetInnerHTML={{ __html: intro }}
      />
      <div className="items-container">
        {items.map(({ title, content, icon }) => (
          <div key={title} className="item">
            <div className="item-icon">
              <img src={icon} alt={title} />
            </div>
            <div className="content">
              <h4 className="g-type-display-4">{title}</h4>
              <p
                className="g-type-body-small"
                dangerouslySetInnerHTML={{ __html: content }}
              />
            </div>
          </div>
        ))}
      </div>
    </div>
  )
}
