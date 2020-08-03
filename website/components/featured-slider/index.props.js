import React from 'react'
import PropTypes from 'prop-types'
import FeaturedSlider from './dist/index.js'

function FeaturedSliderProps(props) {
  return <FeaturedSlider {...props} />
}

FeaturedSliderProps.propTypes = {
  theme: PropTypes.oneOf(['light', 'dark']),
  brand: PropTypes.oneOf([
    'hashicorp',
    'terraform',
    'vault',
    'consul',
    'nomad',
    'packer',
    'vagrant'
  ]),
  features: PropTypes.arrayOf(
    PropTypes.shape({
      logo: PropTypes.shape({
        url: PropTypes.string,
        alt: PropTypes.string
      }),
      image: PropTypes.shape({
        url: PropTypes.string,
        alt: PropTypes.string
      }),
      heading: PropTypes.string,
      content: PropTypes.string,
      link: PropTypes.shape({
        text: PropTypes.string,
        url: PropTypes.string,
        type: PropTypes.oneOf(['anchor', 'inbound', 'outbound'])
      })
    })
  )
}

export default FeaturedSliderProps
