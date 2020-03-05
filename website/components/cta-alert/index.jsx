import Button from '@hashicorp/react-button'
import InlineSvg from '@hashicorp/react-inline-svg'
import CloseIcon from './img/close-icon.svg?include'

export default function CtaAlert({ tag, url, message }) {
  return (
    <div className="g-cta-alert">
      <span className="tag g-type-label-strong">{tag}</span>
      <Button
        url={url}
        title={message}
        linkType="outbound"
        theme={{ variant: 'tertiary-neutral', background: 'dark' }}
      />
      <button
        className="close-button"
        onClick={() =>
          (document.querySelector('.g-cta-alert').style.display = 'none')
        }
      >
        <InlineSvg src={CloseIcon} />
      </button>
    </div>
  )
}
