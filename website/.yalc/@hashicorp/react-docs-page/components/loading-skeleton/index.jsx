import Placeholder from '@hashicorp/react-placeholder'
import s from './style.module.css'

export default function LoadingSkeleton() {
  return (
    <Placeholder>
      {(Box) => (
        <div className={s.wrapper}>
          <div className={s.sidebar}>
            <Box width="7ch" height="0.75rem" marginBottom="4px" />
            <Box height="30px" />
            <Box height="30px" />
            <Box height="1rem" marginBottom="16px" repeat={3} />
            <Box height="1px" />
            <Box height="1rem" marginBottom="16px" repeat={5} />
            <Box height="1px" />
            <Box height="1rem" marginBottom="16px" repeat={3} />
          </div>
          <div className={s.content}>
            {/* Old Version Notice */}
            <Box height="40px" />
            {/* Search */}
            <Box height="40px" marginBottom="56px" />
            {/* H1 and Jump to Section */}
            <Box
              width="16ch"
              fontSize="2.5rem"
              height="40px"
              marginBottom="24px"
            />
            <Box
              width="16ch"
              height="1rem"
              marginBottom="20px"
              display="inline-block"
            />
            {/* Content */}
            <Box lines={['80ch', '80ch', '65ch']} prose />
            <Box lines={['70ch']} prose />
            <Box lines={['80ch', '80ch', '80ch', '35ch']} prose />
            <Box height="100px" margin="20px 0" display="inline-block" />
          </div>
        </div>
      )}
    </Placeholder>
  )
}
