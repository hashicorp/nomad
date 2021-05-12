import path from 'path'
import { serialize } from 'next-mdx-remote/serialize'
import markdownDefaults from '@hashicorp/nextjs-scripts/markdown'
import grayMatter from 'gray-matter'

async function renderPageMdx(
  mdxFileString,
  { mdxContentHook = (c) => c, remarkPlugins = [], scope } = {}
) {
  const { data: frontMatter, content: rawContent } = grayMatter(mdxFileString)
  const content = mdxContentHook(rawContent)
  const mdxSource = await serialize(content, {
    mdxOptions: markdownDefaults({
      resolveIncludes: path.join(process.cwd(), 'content/partials'),
      addRemarkPlugins: remarkPlugins,
    }),
    scope,
  })
  return { mdxSource, frontMatter }
}

export default renderPageMdx
