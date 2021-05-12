var fs = require('fs')
var path = require('path')

/**
 *
 * Given navData and a content directory,
 * return an array of .mdx files that are NOT included
 * in the navData. If all files are included,
 * returns an empty array.
 *
 * @param {*} navData - array of navData nodes
 * @param {*} contentDir - path from the cwd to the content directory
 * @returns {Array} - array of file paths which are present in the content directory,
 * but missing from navData. Returns an empty array if all content files are included in navData.
 */
async function validateUnlinkedContent(navData, contentDir) {
  // Flatten navData to simplify filtering of missing files
  const navDataFlat = flattenNodes(navData)
  // Read all files in the content directory
  const files = await getFiles(path.join(process.cwd(), contentDir))
  // Filter out content files that are already
  // included in nav-data.json
  const missingPages = files
    // Ignore non-.mdx files
    .filter((filePath) => {
      return path.extname(filePath) == '.mdx'
    })
    // Transform the filePath into an expected route
    .map((filePath) => {
      // Get the relative filepath, that's what we'll see in the route
      const contentDirPath = path.join(process.cwd(), contentDir)
      const relativePath = path.relative(contentDirPath, filePath)
      // Remove extensions, these will not be in routes
      const pathNoExt = relativePath.replace(/\.mdx$/, '')
      // Resolve /index routes, these will not have /index in their path
      const routePath = pathNoExt.replace(/\/?index$/, '')
      return routePath
    })
    // Determine if there is a match in nav-data.
    // If there is no match, then this is an unlinked content file.
    .filter((pathToMatch) => {
      // If it's the root path index page, we know
      // it'll be rendered (hard-coded into docs-page/server.js)
      const isIndexPage = pathToMatch === ''
      if (isIndexPage) return false
      // Otherwise, needs a path match in nav-data
      const matches = navDataFlat.filter(({ path }) => path == pathToMatch)
      return matches.length == 0
    })
  return missingPages
}

function flattenNodes(nodes) {
  return nodes.reduce((acc, n) => {
    if (!n.routes) return acc.concat(n)
    return acc.concat(flattenNodes(n.routes))
  }, [])
}

async function getFiles(dir) {
  const entries = await fs.promises.readdir(dir, { withFileTypes: true })
  const files = await Promise.all(
    entries.map((entry) => {
      const res = path.resolve(dir, entry.name)
      return entry.isDirectory() ? getFiles(res) : res
    })
  )
  return files.flat()
}

module.exports = validateUnlinkedContent
