const fs = require('fs')
const path = require('path')

async function validateFilePaths(navNodes, localDir) {
  //  Clone the nodes, and validate each one
  return await Promise.all(
    navNodes.slice(0).map(async (navNode) => {
      return await validateNode(navNode, localDir)
    })
  )
}

async function validateNode(navNode, localDir) {
  // Ignore remote leaf nodes, these already
  // have their content file explicitly defined
  // (note: remote leaf nodes are currently only used
  // for Packer plugin documentation)
  if (navNode.remoteFile) return navNode
  // Handle local leaf nodes
  if (navNode.path) {
    const indexFilePath = path.join(navNode.path, 'index.mdx')
    const namedFilePath = `${navNode.path}.mdx`
    const hasIndexFile = fs.existsSync(
      path.join(process.cwd(), localDir, indexFilePath)
    )
    const hasNamedFile = fs.existsSync(
      path.join(process.cwd(), localDir, namedFilePath)
    )
    if (!hasIndexFile && !hasNamedFile) {
      throw new Error(
        `Could not find file to match path "${navNode.path}". Neither "${namedFilePath}" or "${indexFilePath}" could be found.`
      )
    }
    if (hasIndexFile && hasNamedFile) {
      throw new Error(
        `Ambiguous path "${navNode.path}". Both "${namedFilePath}" and "${indexFilePath}" exist. Please delete one of these files.`
      )
    }
    const filePath = path.join(
      localDir,
      hasIndexFile ? indexFilePath : namedFilePath
    )
    return { ...navNode, filePath }
  }
  //  Handle local branch nodes
  if (navNode.routes) {
    const routesWithFilePaths = await validateFilePaths(
      navNode.routes,
      localDir
    )
    return { ...navNode, routes: routesWithFilePaths }
  }
  // Return all other node types unmodified
  return navNode
}

module.exports = validateFilePaths
