import useOverflowRef from './useOverflowRef'
import useStuckRef from './useStuckRef'

export default function useNavRef(deps) {
  const [isStuck, stuckRef] = useStuckRef()
  const [hasOverflow, overflowRef] = useOverflowRef()

  return [
    isStuck,
    hasOverflow,
    function navRef(target) {
      stuckRef(target, deps)
      overflowRef(target, deps)
    },
  ]
}
