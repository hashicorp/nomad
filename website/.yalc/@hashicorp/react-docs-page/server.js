import fs from 'fs'
import path from 'path'
import validateFilePaths from '@hashicorp/react-docs-sidenav/utils/validate-file-paths'
import validateRouteStructure from '@hashicorp/react-docs-sidenav/utils/validate-route-structure'
import validateUnlinkedContent from '@hashicorp/react-docs-sidenav/utils/validate-unlinked-content'
import {
  loadVersionListFromManifest,
  loadVersionedDocument,
  loadVersionedNavData,
  getVersionFromPath,
} from '@hashicorp/versioned-docs/server'
import moize from 'moize'
import { normalizeVersion } from '@hashicorp/versioned-docs/client'
import renderPageMdx from './render-page-mdx'

const cachedLoadVersionNavData = moize(loadVersionedNavData)

// So far, we have a pattern of using a common value for
// docs catchall route parameters: route/[[...page]].jsx.
// This default parameter ID captures that pattern.
// It can be overridden via options.
const DEFAULT_PARAM_ID = 'page'

async function generateStaticPaths({
  navDataFile,
  localContentDir,
  paramId = DEFAULT_PARAM_ID,
  product,
  basePath,
  currentVersion,
}) {
  let navData

  // This code path handles versioned docs integration, which is currently gated behind the ENABLE_VERSIONED_DOCS env var
  if (
    process.env.ENABLE_VERSIONED_DOCS &&
    process.env.VERCEL_ENV === 'preview'
  ) {
    // Fetch and parse navigation data
    navData = (
      await cachedLoadVersionNavData(
        product.slug,
        basePath,
        normalizeVersion(currentVersion)
      )
    ).navData
  } else {
    navData = await resolveNavData(navDataFile, localContentDir)
  }

  return getPathsFromNavData(navData, paramId)
}

async function resolveNavData(filePath, localContentDir) {
  const navDataFile = path.join(process.cwd(), filePath)
  const navDataRaw = JSON.parse(fs.readFileSync(navDataFile, 'utf8'))
  const withFilePaths = await validateNavData(navDataRaw, localContentDir)
  return withFilePaths
}

async function getPathsFromNavData(
  navDataResolved,
  paramId = DEFAULT_PARAM_ID
) {
  //  Transform navigation data into path arrays
  const pagePathArrays = getPathArraysFromNodes(navDataResolved)
  // Include an empty array for the "/" index page path
  const allPathArrays = [[]].concat(pagePathArrays)
  const paths = allPathArrays.map((p) => ({ params: { [paramId]: p } }))
  return paths
}

async function generateStaticProps({
  navDataFile,
  localContentDir,
  params,
  product,
  mainBranch = 'main',
  remarkPlugins = [],
  scope, // optional, I think?
  paramId = DEFAULT_PARAM_ID,
  basePath,
  currentVersion,
}) {
  const mdxRenderer = (mdx) =>
    renderPageMdx(mdx, {
      productName: product.name,
      remarkPlugins,
      scope,
    })

  // Build the currentPath from page parameters
  const currentPath = params[paramId] ? params[paramId].join('/') : ''

  let versions = []

  // This code path handles versioned docs integration, which is currently gated behind the ENABLE_VERSIONED_DOCS env var
  if (process.env.ENABLE_VERSIONED_DOCS) {
    const versionFromPath = getVersionFromPath(params[paramId])

    const currentVersionNormalized = normalizeVersion(currentVersion)

    versions = await loadVersionListFromManifest(currentVersionNormalized)

    // Only load docs content from the DB if we're in production or there's an explicit version in the path
    // Preview and dev environments will read the "latest" content from the filesystem
    if (process.env.VERCEL_ENV === 'preview' || versionFromPath) {
      const pagePathToLoad = versionFromPath
        ? [basePath, ...(params[paramId] ?? [])].join('/')
        : [basePath, currentVersionNormalized, ...(params[paramId] ?? [])].join(
            '/'
          )

      let doc
      const [{ mdxSource }, navData] = await Promise.all([
        loadVersionedDocument(product.slug, pagePathToLoad).then(
          (docResult) => {
            doc = docResult
            return mdxRenderer(docResult.markdownSource)
          }
        ),
        cachedLoadVersionNavData(
          product.slug,
          basePath,
          versionFromPath ?? currentVersionNormalized
        ),
      ])

      // Construct the githubFileUrl, used for "Edit this page" link
      const githubFileUrl = `https://github.com/hashicorp/${product.slug}/blob/${doc.gitRef}/website/content/${doc.filePath}`

      return {
        versions,
        currentPath,
        frontMatter: doc.metadata,
        githubFileUrl,
        mdxSource,
        navData: navData.navData,
      }
    }
  }

  //  Read in the nav data, and resolve local filePaths
  const navData = await resolveNavData(navDataFile, localContentDir)
  //  Get the navNode that matches this path
  const navNode = getNodeFromPath(currentPath, navData, localContentDir)
  //  Read in and process MDX content from the navNode's filePath
  const mdxFile = path.join(process.cwd(), navNode.filePath)
  const mdxString = fs.readFileSync(mdxFile, 'utf8')
  const { mdxSource, frontMatter } = await mdxRenderer(mdxString)

  // Construct the githubFileUrl, used for "Edit this page" link
  const githubFileUrl = `https://github.com/hashicorp/${product.slug}/blob/${mainBranch}/website/${navNode.filePath}`
  // Return all the props
  return {
    currentPath,
    frontMatter,
    githubFileUrl,
    mdxSource,
    navData,
    versions,
  }
}

async function validateNavData(navData, localContentDir) {
  const withFilePaths = await validateFilePaths(navData, localContentDir)
  // Validate unlinked content checks for content files that are NOT
  // included in the provided navData. This requires filesystem access,
  // similar to validateFilePaths
  const unlinkedRoutes = await validateUnlinkedContent(navData, localContentDir)
  if (unlinkedRoutes.length > 0) {
    const COLOR_RESET = '\x1b[0m'
    const COLOR_RED = '\x1b[31m'
    const jsonList = JSON.stringify(unlinkedRoutes, null, 2)
    throw new Error(
      `\n${COLOR_RED}Error: Unlinked pages found in the ${localContentDir} directory.\n\nPlease add these paths to the "${localContentDir}" nav data file, or remove the .mdx files. If you want the pages to exist but not be linked in the navigation, you can add a "hidden" property to the page object in the navigation structure.\n\n${jsonList}${COLOR_RESET}\n\n`
    )
  }
  // Note: validateRouteStructure returns navData with additional __stack properties,
  // which detail the path we've inferred for each branch and node
  // (branches do not have paths defined explicitly, so we need to infer them)
  // We don't actually need the __stack properties for rendering, they're just
  // used in validation, so we don't use the output of this function.
  validateRouteStructure(withFilePaths)
  // Return the resolved, validated navData
  return withFilePaths
}

function getNodeFromPath(pathToMatch, navData, localContentDir) {
  // If there is no path array, we return a
  // constructed "home page" node. This is just to
  // provide authoring convenience to not have to define
  // this node. However, we could ask for this node to
  // be explicitly defined in `navData` (and if it isn't,
  // then we'd render a 404 for the root path)
  const isLandingPage = pathToMatch === ''
  if (isLandingPage) {
    return {
      filePath: path.join(localContentDir, 'index.mdx'),
    }
  }
  //  If it's not a landing page, then we search
  // through our navData to find the node with a path
  // that matches the pathArray we're looking for.
  function flattenRoutes(nodes) {
    return nodes.reduce((acc, n) => {
      if (!n.routes) return acc.concat(n)
      return acc.concat(flattenRoutes(n.routes))
    }, [])
  }
  const allNodes = flattenRoutes(navData)
  const matches = allNodes.filter((n) => n.path === pathToMatch)
  // Throw an error for missing files - if this happens,
  // we might have an issue with `getStaticPaths` or something
  if (matches.length === 0) {
    throw new Error(`Missing resource to match "${pathToMatch}"`)
  }
  // Throw an error if there are multiple matches
  // If this happens, there's likely an issue in the
  // content source repo
  if (matches.length > 1) {
    throw new Error(
      `Ambiguous path matches for "${pathToMatch}". Found:\n\n${JSON.stringify(
        matches
      )}`
    )
  }
  //  Otherwise, we have exactly one match,
  //  and we can return the filePath off of it
  return matches[0]
}

function getPathArraysFromNodes(navNodes) {
  const slugs = navNodes.reduce((acc, navNode) => {
    // Individual items have a path, these should be added
    if (navNode.path) return acc.concat([navNode.path.split('/')])
    // Category items have child routes, these should all be added
    if (navNode.routes)
      return acc.concat(getPathArraysFromNodes(navNode.routes))
    // All other node types (dividers, external links) can be ignored
    return acc
  }, [])
  return slugs
}

// We currently export most utilities individually,
// since we have cases such as Packer remote plugin docs
// where we want to re-use these utilities to build
// getStaticPaths and getStaticProps functions that
// fall outside the use case of local-only content
export {
  generateStaticPaths,
  generateStaticProps,
  getNodeFromPath,
  getPathsFromNavData,
  validateNavData,
  validateFilePaths,
}
