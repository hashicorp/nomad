import FeaturedSlider from '../featured-slider'

export default function FeaturedSliderSection({ heading, features }) {
  return (
    <section className="g-featured-slider-section">
      <div className="g-grid-container">
        <h2 className="g-type-display-2">{heading}</h2>
        <FeaturedSlider theme="dark" brand="nomad" features={features} />
      </div>
    </section>
  )
}
