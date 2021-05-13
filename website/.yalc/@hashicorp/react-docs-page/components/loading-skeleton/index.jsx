import Placeholder from '@hashicorp/react-placeholder'
import s from './style.module.css'

export default function LoadingSkeleton() {
  return (
    <Placeholder>
      {(Box) => (
        <>
          <div className={`${s.menuBar} ${s.hideDesktop} g-grid-container`}>
            <Box height="26px" width="200px" marginBottom={0} />
            <Box height="24px" width="24px" marginBottom={0} />
          </div>
          {/* Old Version Notice / mobile nav */}
          <div
            className={`${s.oldVersionNotice} ${s.hideMobile} g-grid-container`}
          >
            <Box height="44px" marginTop="16px" />
          </div>
          <div className={`${s.wrapper} g-grid-container`}>
            <div className={`${s.sidebar} ${s.hideMobile}`}>
              <Box width="7ch" height="0.75rem" marginBottom="4px" />
              <Box height="40px" />
              <Box height="36px" />
              <Box height="1rem" marginBottom="16px" repeat={3} />
              <Box height="1px" />
              <Box height="1rem" marginBottom="16px" repeat={5} />
              <Box height="1px" />
              <Box height="1rem" marginBottom="16px" repeat={3} />
            </div>
            <div className={s.content}>
              {/* Search */}
              <div className={`${s.search} ${s.hideMobile}`}>
                <Box height="40px" marginBottom="56px" />
              </div>
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
        </>
      )}
    </Placeholder>
  )
}
