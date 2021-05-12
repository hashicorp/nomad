import Alert from '@hashicorp/react-alert'
import { useRouter } from 'next/router'
import {
  getVersionFromPath,
  removeVersionFromPath,
} from '@hashicorp/versioned-docs/client'
import s from './style.module.css'
import useIsMobile from '../../use-is-mobile'

export default function VersionAlert({ product }) {
  const router = useRouter()
  const isMobile = useIsMobile()
  const versionInPath = getVersionFromPath(router.asPath)

  if (!versionInPath) return null

  return (
    <div className={s.wrapper}>
      <Alert
        url={removeVersionFromPath(router.asPath)}
        tag={`old version ${isMobile ? `(${versionInPath})` : ''}`}
        text={
          isMobile
            ? `Click to view latest`
            : `You're looking at documentation for ${product} ${versionInPath}. Click here to view the latest content.`
        }
        state="warning"
        textColor="dark"
      />
    </div>
  )
}
