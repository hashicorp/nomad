import { useState, useEffect, useLayoutEffect } from 'react'

const useSafeLayoutEffect =
  typeof window === 'undefined' ? useEffect : useLayoutEffect

// TODO: de-dupe from learn implementation: https://github.com/hashicorp/learn/blob/master/lib/hooks/use-window-size.js
//  https://usehooks.com/useWindowSize/
function useWindowSize() {
  // Initialize state with undefined width/height so server and client renders match
  // Learn more here: https://joshwcomeau.com/react/the-perils-of-rehydration/
  const [windowSize, setWindowSize] = useState({
    width: undefined,
    height: undefined,
  })

  useSafeLayoutEffect(() => {
    // Handler to call on window resize
    function handleResize() {
      // Set window width/height to state
      setWindowSize({
        width: window.innerWidth,
        height: window.innerHeight,
      })
    }

    // Add event listener
    window.addEventListener('resize', handleResize)

    // Call handler right away so state gets updated with initial window size
    handleResize()

    // Remove event listener on cleanup
    return () => window.removeEventListener('resize', handleResize)
  }, []) // Empty array ensures that effect is only run on mount

  return windowSize
}

export default function useIsMobile() {
  const { width: windowWidth } = useWindowSize()

  return windowWidth < 940
}
