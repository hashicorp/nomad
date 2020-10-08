import path from 'path'
import matter from 'gray-matter'
import readdirp from 'readdirp'
import highlight from '@mapbox/rehype-prism'
import renderToString from 'next-mdx-remote/render-to-string'
import {
  anchorLinks,
  includeMarkdown,
  paragraphCustomAlerts,
  typography,
} from '@hashicorp/remark-plugins'
import { readAllFrontMatter, readContent } from './mdx'
import { MDX_COMPONENTS } from './mdx-components'

const GITHUB_CONTENT_REPO = 'hashicorp/nomad'

export async function generateStaticProps(subpath, params) {
  const slug = [subpath, ...(params.slug || [])].join('/')
  const url = `/${slug}`
  const mdxPath = `content/${slug}.mdx`
  const indexMdxPath = `content/${slug}/index.mdx`

  const [mdxContent, indexMdxContent] = await Promise.all([
    readContent(`${process.cwd()}/${mdxPath}`),
    readContent(`${process.cwd()}/${indexMdxPath}`),
  ])
  const sidenavData = await readAllFrontMatter(
    `${process.cwd()}/content/${subpath}`
  )

  const { content, data: frontMatter } = matter(mdxContent || indexMdxContent)
  const renderedContent = await renderToString(content, {
    components: MDX_COMPONENTS,
    mdxOptions: {
      remarkPlugins: [
        [
          includeMarkdown,
          { resolveFrom: path.join(process.cwd(), 'content/partials') },
        ],
        anchorLinks,
        paragraphCustomAlerts,
        typography,
      ],
      rehypePlugins: [[highlight, { ignoreMissing: true }]],
    },
  })

  return {
    props: {
      renderedContent,
      frontMatter,
      resourceUrl: `https://github.com/${GITHUB_CONTENT_REPO}/blob/master/website/${
        mdxContent ? mdxPath : indexMdxPath
      }`,
      url,
      sidenavData,
    },
  }
}

export async function generateStaticPaths(subpath) {
  const mdxFiles = await readdirp.promise(
    `${process.cwd()}/content/${subpath}`,
    {
      fileFilter: '*.mdx',
    }
  )

  return {
    paths: mdxFiles.map(({ path: mdxPath }) => ({
      params: {
        slug: mdxPath
          .replace(/\.mdx$/, '')
          .replace(/(^|\/)index$/, '')
          .split('/'),
      },
    })),
    fallback: false,
  }
}
