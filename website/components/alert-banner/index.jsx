import React, { Component } from 'react'
import CloseIcon from './img/close-icon.svg?include'
import cookie from 'js-cookie'
import slugify from 'slugify'
import fragment from './fragment.graphql'
import Button from '@hashicorp/react-button'
import InlineSvg from '@hashicorp/react-inline-svg'

// "Ejected" version of @hashicorp/react-alert-banner
// Needed this to customize the look of the banner

class AlertBanner extends Component {
  constructor(props) {
    super(props)

    this.name = props.name || slugify(props.text, { lower: true })
    this.state = { show: true }
    this.banner = React.createRef()
  }

  render() {
    const { url, tag, theme, linkText } = { ...this.props }
    const classes = ['g-alert-banner']

    // add theme class
    classes.push(theme ? theme : 'default')

    // add has-large-tag class if needed
    if (tag.length > 3) classes.push('has-large-tag')

    // add show class based on state
    if (this.state.show) classes.push('show')

    return (
      <div className={classes.join(' ')} ref={this.banner}>
        <a
          href={url}
          onClick={() => {
            this.trackEvent('click')
          }}
        >
          {' '}
        </a>
        <div className="g-container">
          <div className="tag g-type-label">
            <span>{tag}</span>
          </div>
          <div className="text g-type-body-small-strong">
            {linkText && (
              <Button
                url={url}
                title={linkText}
                linkType="outbound"
                theme={{ variant: 'tertiary-neutral', background: 'dark' }}
              />
            )}
          </div>
        </div>
        <span
          className="close"
          onClick={() => {
            this.onClose()
          }}
        >
          <InlineSvg src={CloseIcon} />
        </span>
      </div>
    )
  }

  componentDidMount() {
    // if cookie isn't set, show the component
    this.setState({ show: cookie.get(`banner_${this.name}`) ? false : true })
  }

  onClose() {
    // animate closed by setting height so
    // it's not 'auto' and then set to zero
    this.banner.current.style.height = `${this.banner.scrollHeight}px`
    window.setTimeout(() => {
      this.banner.current.style.height = '0'
    }, 1)

    // set the cookie so this banner doesn't show up anymore
    const name = `banner_${this.name}`
    cookie.set(name, 1)

    this.trackEvent('close')
  }

  trackEvent(type) {
    if (window.analytics) {
      const { tag, theme, text, linkText } = { ...this.props }

      window.analytics.track(type.charAt(0).toUpperCase() + type.slice(1), {
        category: 'Alert Banner',
        label: `${text} - ${linkText} | ${type}`,
        tag: tag,
        theme: theme
      })
    }
  }
}

AlertBanner.fragmentSpec = { fragment }

export default AlertBanner
