function addIsActiveToNodes(navNodes, currentPath, pathname) {
  return navNodes
    .slice()
    .map((node) => addIsActiveToNode(node, currentPath, pathname))
}

function addIsActiveToNode(navNode, currentPath, pathname) {
  // If it's a node with child routes, return true
  // if any of the child routes are active
  if (navNode.routes) {
    const routesWithActive = addIsActiveToNodes(
      navNode.routes,
      currentPath,
      pathname
    )
    const isActive = routesWithActive.filter((r) => r.__isActive).length > 0
    return { ...navNode, routes: routesWithActive, __isActive: isActive }
  }
  // If it's a node with a path value,
  // return true if the path is a match
  if (navNode.path) {
    const isActive = navNode.path === currentPath
    return { ...navNode, __isActive: isActive }
  }
  // If it's a direct link,
  // return true if the path matches the router.pathname
  if (navNode.href) {
    const isActive = navNode.href === pathname
    return { ...navNode, __isActive: isActive }
  }
  // Otherwise, it's a divider, so return unmodified
  return navNode
}

export default addIsActiveToNodes
