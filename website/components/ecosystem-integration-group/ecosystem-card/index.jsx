import s from './style.module.css'
import Image from '@hashicorp/react-image'
import SmartLink from 'components/smart-link'

export default function EcosystemCard({
  companyName,
  integrationType,
  integrationUrl,
  companyLogoUrl,
}) {
  if (!['Partner', 'Community', 'HashiCorp'].includes(integrationType))
    throw new Error(
      'integrationType should be one of these: Partner, Community, or HashiCorp'
    )

  return (
    <SmartLink
      className={s.ecosystemCard}
      href={integrationUrl}
      prefetch={false}
    >
      <div className={s.companyInfo}>
        <div className={s.companyLogo}>
          <Image url={companyLogoUrl} alt={`${companyName} Logo`} />
        </div>
        <div className={s.companyNameLabel}>{companyName}</div>
      </div>

      <div className={s.integrationTypeLabel}>
        <span>{integrationType}</span>
        <span> integration</span>
      </div>
    </SmartLink>
  )
}
