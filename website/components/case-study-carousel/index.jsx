import { useState } from 'react'

import Carousel from 'nuka-carousel'
import CaseSlide from './case-study-slide'
import Image from '@hashicorp/react-image'
import InlineSvg from '@hashicorp/react-inline-svg'
import ActiveControlDot from './img/active-control-dot.svg?include'
import InactiveControlDot from './img/inactive-control-dot.svg?include'
import LeftArrow from './img/left-arrow-control.svg?include'
import RightArrow from './img/right-arrow-control.svg?include'

export default function CaseStudyCarousel({ caseStudies, title }) {
  const [slideIndex, setSlideIndex] = useState(0)
  const caseStudySlides = caseStudies.map(caseStudy => (
    <CaseSlide key={caseStudy.quote} caseStudy={caseStudy} />
  ))
  function renderControls() {
    return (
      <div className="carousel-controls">
        {caseStudies.map((caseStudy, stableIdx) => {
          return (
            <button
              key={caseStudy.quote}
              className="carousel-controls-button"
              onClick={() => setSlideIndex(stableIdx)}
            >
              <InlineSvg
                src={
                  slideIndex === stableIdx
                    ? ActiveControlDot
                    : InactiveControlDot
                }
              />
            </button>
          )
        })}
      </div>
    )
  }

  function sideControls(icon, direction, disabled) {
    return (
      <button className="side-control" onClick={direction} disabled={disabled}>
        <InlineSvg src={icon} />
      </button>
    )
  }

  return (
    <section className="g-case-carousel">
      <h2 className="g-type-display-2">{title}</h2>
      <Carousel
        cellAlign="left"
        heightMode="max"
        slideIndex={slideIndex}
        slidesToShow={1}
        autoGenerateStyleTag
        renderBottomCenterControls={() => renderControls()}
        renderCenterLeftControls={({ previousSlide }) => {
          const disabled = slideIndex === 0
          return sideControls(LeftArrow, previousSlide, disabled)
        }}
        renderCenterRightControls={({ nextSlide }) => {
          const disabled = slideIndex === caseStudySlides.length - 1
          return sideControls(RightArrow, nextSlide, disabled)
        }}
        afterSlide={slideIndex => setSlideIndex(slideIndex)}
      >
        {caseStudySlides}
      </Carousel>

      <div className="background-section">
        <div className="mono-logos">
          {caseStudies.map(item => (
            <Image
              key={item.company.name}
              url={item.company.monochromaticLogo}
              alt={item.company.name}
            />
          ))}
        </div>
      </div>
    </section>
  )
}
