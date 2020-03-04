import { useState } from 'react'
import { isIE } from 'react-device-detect'

import Carousel from 'nuka-carousel'
import CaseSlide from './case-study-slide'
import Image from '@hashicorp/react-image'
import InlineSvg from '@hashicorp/react-inline-svg'
import ActiveControlDot from './img/active-control-dot.svg?include'
import InactiveControlDot from './img/inactive-control-dot.svg?include'
import LeftArrow from './img/left-arrow-control.svg?include'
import RightArrow from './img/right-arrow-control.svg?include'

export default function CaseStudyCarousel({
  caseStudies,
  title,
  featuredLogos
}) {
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

  function sideControls(icon, direction) {
    return (
      <button className="side-control" onClick={direction}>
        <InlineSvg src={icon} />
      </button>
    )
  }

  return (
    <section className="g-case-carousel">
      <h2 className="g-type-display-2">{title}</h2>
      {!isIE ? (
        <Carousel
          cellAlign="left"
          wrapAround={true}
          heightMode="current"
          slideIndex={slideIndex}
          slidesToShow={1}
          autoGenerateStyleTag
          renderBottomCenterControls={() => renderControls()}
          renderCenterLeftControls={({ previousSlide }) => {
            return sideControls(LeftArrow, previousSlide)
          }}
          renderCenterRightControls={({ nextSlide }) => {
            return sideControls(RightArrow, nextSlide)
          }}
          afterSlide={slideIndex => setSlideIndex(slideIndex)}
        >
          {caseStudySlides}
        </Carousel>
      ) : null}
      <div className="background-section">
        <div className="mono-logos">
          {featuredLogos.map(featuredLogo => (
            <Image
              key={featuredLogo.url}
              url={featuredLogo.url}
              alt={featuredLogo.companyName}
            />
          ))}
        </div>
      </div>
    </section>
  )
}
