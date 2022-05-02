import Subnav from '@hashicorp/react-subnav'
import { useRouter } from 'next/router'
import s from './style.module.css'

export default function NomadSubnav({ menuItems }) {
  const router = useRouter()
  return (
    <Subnav
      className={s.subnav}
      hideGithubStars={true}
      titleLink={{
        text: 'HashiCorp Nomad',
        url: '/',
      }}
      ctaLinks={[
        { text: 'GitHub', url: 'https://www.github.com/hashicorp/nomad' },
        {
          text: 'Download',
          url: '/downloads',
          theme: {
            brand: 'nomad',
          },
        },
      ]}
      currentPath={router.asPath}
      menuItemsAlign="right"
      menuItems={menuItems}
      constrainWidth
      matchOnBasePath
    />
  )
}
