import { useCallback, useState } from 'react'

const IntersectionObserver =
  (typeof window !== 'undefined' && window.IntersectionObserver) || null
const intersectionOpts = { threshold: [1] }

/*

Stuck-ness is determined by whether a sticky target element has intersected the
top boundary of its viewport.

Stuck-ness is checked on intersection observation.

IntersectionObserver Compatibility (https://developer.mozilla.org/en-US/docs/Web/API/IntersectionObserver#Browser_compatibility):

- Chrome 51+
- Edge 15+
- Firefox 55+
- Safari 12.1+

Internet Explorer 11 gracefully skips over this functionality, as it does not
support `IntersectionObserver` or CSS `position: sticky`.

*/

export default function useStuckRef(deps) {
  const [isStuck, setStuck] = useState(false)

  const stuckRef = useCallback((target) => {
    if (target && IntersectionObserver) {
      const intersectionObserver = new IntersectionObserver(([entry]) => {
        const nowIsStuck = entry.intersectionRatio < 1

        if (isStuck !== nowIsStuck) setStuck(nowIsStuck)
      }, intersectionOpts)

      intersectionObserver.observe(target)

      return intersectionObserver.disconnect.bind(intersectionObserver)
    }
  }, deps)

  return [isStuck, stuckRef]
}
