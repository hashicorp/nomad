import React from 'react'

export default function StatusBar({ theme, active, timing, brand }) {
  return (
    <div className={`progress-bar ${theme}`}>
      <span
        className={`${active ? ' active' : ''} ${brand ? brand : ''}`}
        style={
          active
            ? { animationDuration: `${timing}s` }
            : { animationDuration: '0s' }
        }
      />
    </div>
  )
}
