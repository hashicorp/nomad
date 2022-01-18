import query from './query.graphql'
import ProductSubnav from 'components/subnav'
import Footer from 'components/footer'
import { open } from '@hashicorp/react-consent-manager'

export default function StandardLayout(props: Props): React.ReactElement {
  // const { useCaseNavItems } = props.data
  return (
    <>
      <ProductSubnav />
      {props.children}
      <Footer openConsentManager={open} />
    </>
  )
}

StandardLayout.rivetParams = {
  query,
  dependencies: [],
}

interface Props {
  children: React.ReactChildren
  data: {
    useCaseNavItems: Array<{ url: string; text: string }>
  }
}
