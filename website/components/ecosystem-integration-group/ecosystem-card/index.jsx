import s from './style.module.css'
import Image from '@hashicorp/react-image'
import SmartLink from 'components/smart-link'

export default function EcosystemCard({
  companyName,
  integrationUrl,
  companyLogoUrl,
}) {
  return (
    <SmartLink
      className={s.ecosystemCard}
      href={integrationUrl}
      as={integrationUrl}
      prefetch={false}
    >
      <div className="logo">
        <Image url={companyLogoUrl} alt={`${companyName} Logo`} />
      </div>
      <div className="integration-label">{companyName}</div>
    </SmartLink>
  )
}
