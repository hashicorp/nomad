import MarkdownPage from '@hashicorp/react-markdown-page'
import generateStaticProps from '@hashicorp/react-markdown-page/server'

export default function SecurityPage(staticProps) {
  return <MarkdownPage {...staticProps} product="nomad" />
}

export const getStaticProps = generateStaticProps({
  pagePath: 'content/security.mdx',
})
