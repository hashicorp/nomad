import { useCallback, useState } from 'react'

const MutationObserver =
  (typeof window !== 'undefined' && window.MutationObserver) || null
const mutationOpts = { attributes: true, childList: true, subtree: true }

/*

Overflow is determined by whether the width of a target element’s content is
greater than the width of its layout.

Overflow is checked on viewport resize and on a target element’s DOM mutation.
Both are throttled by animation frame requests.

MutationObserver Compatibility (https://developer.mozilla.org/en-US/docs/Web/API/MutationObserver):

- Chrome 26+
- Edge 12+
- Firefox 14+
- Internet Explorer 11+
- Safari 17+

*/

export default function useOverflowRef(deps) {
  const [[hasOverflow, scrollWidth], setState] = useState([false, 0])

  const overflowRef = useCallback((target) => {
    var viewport
    let requestId
    let mutationObserver

    if (target) {
      viewport = target.ownerDocument.defaultView

      viewport.addEventListener('resize', requestOverflow)

      if (MutationObserver) {
        mutationObserver = new MutationObserver(requestOverflow)

        mutationObserver.observe(target, mutationOpts)
      }

      requestOverflow()

      return cleanup
    }

    function requestOverflow() {
      if (!requestId) requestId = requestAnimationFrame(checkOverflow)
    }

    function checkOverflow() {
      requestId = null

      const nowScrollWidth = target.scrollWidth
      const nowOffsetWidth = target.offsetWidth
      const nowHasOverflow = (scrollWidth || nowScrollWidth) > nowOffsetWidth

      if (hasOverflow !== nowHasOverflow) {
        cleanup()

        setState([nowHasOverflow, nowScrollWidth])
      }
    }

    function cleanup() {
      cancelAnimationFrame(requestId)

      viewport.removeEventListener('resize', requestOverflow)

      if (MutationObserver) mutationObserver.disconnect()
    }
  }, deps)

  return [hasOverflow, overflowRef]
}
